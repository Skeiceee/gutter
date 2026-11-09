package app

import (
	"context"
	"image/color"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/backend"
	"gutter/internal/cursor"
	"gutter/internal/ocr"
	"gutter/internal/screen"
	"gutter/internal/theme"
	"gutter/internal/ui"
)

type gutterApp struct {
	window fyne.Window
	prefs  fyne.Preferences

	comic      *backend.Comic
	pageIndex  int
	half       int  // mitad en orden de lectura: 0 = primera, 1 = segunda
	spread     bool // la página actual es horizontal
	singlePage bool
	manga      bool
	loading    bool

	pageSeq atomic.Uint64
	openSeq atomic.Uint64

	// Cancelación de la carga actual y del prefetch. Son contextos separados
	// para que el prefetch no herede el callback de progreso de la página.
	pageCancel     context.CancelFunc
	prefetchCancel context.CancelFunc

	// Última página realmente dibujada. Permite restaurar la vista si el
	// usuario cancela una carga larga (por ejemplo, un salto en RAR sólido).
	shownComic  *backend.Comic
	shownPage   int
	shownHalf   int
	shownSpread bool

	// Progreso del recorrido secuencial de un RAR sólido.
	readScanned atomic.Int64
	readTotal   atomic.Int64

	logo             fyne.Resource
	pageImage        *canvas.Image
	translationImage *canvas.Image
	pageCanvas       *fyne.Container
	pageLayout       *ui.PageLayout
	scroll           *ui.ScrollSinSombras
	emptyState       *fyne.Container
	root             *fyne.Container
	bottomBar        fyne.CanvasObject
	bottomShadow     *canvas.LinearGradient
	progress         *widget.Label
	zoomLabel        *widget.Label
	prevButton       *widget.Button
	nextButton       *widget.Button

	// Controles que reflejan el estado de lectura guardado por cómic.
	singleCheck     *widget.Check
	direction       *widget.Select
	syncingControls bool

	recents []recentComic

	// Caché persistente compartida por todos los cómics. Se crea al arrancar y
	// se pasa al backend en cada apertura.
	diskCache *backend.DiskCache

	// Marcador de página fallida. Sustituye al diálogo modal: el usuario puede
	// seguir navegando y reintentar sin cerrar una ventana.
	pageErrorState  *fyne.Container
	pageErrorLabel  *widget.Label
	pageErrorDetail *widget.Label
	pageFailed      bool

	// Geometría de la última ventana en modo normal. Al cerrar en pantalla
	// completa se guarda esta, no el tamaño del monitor.
	lastWindowedSize fyne.Size

	// Rectángulo transparente que oculta el cursor en modo limpio.
	cursorBlanker *cursor.CleanCursor

	// Menú principal de la ventana. Se retira en modo limpio para que la
	// pantalla completa muestre solo la página, y se restaura al salir.
	mainMenu *fyne.MainMenu

	// HUD flotante: indicador de página en pantalla completa y selector G.
	pageBadge        *floatingPill
	pageBadgeOverlay *fyne.Container
	gotoBadge        *floatingPill
	gotoBadgeOverlay *fyne.Container
	gotoMode         bool
	gotoBuffer       string
	gotoError        string

	// Diálogo modal actual. Garantiza que solo haya uno abierto a la vez,
	// aunque el usuario pulse un atajo repetidamente.
	activeDialog dialog.Dialog

	// Traducción bajo demanda. La caché vive solo en memoria y se indexa por
	// cómic, página, mitad, modo y configuración de traducción.
	translationVisible bool
	translating        bool
	translationCache   map[string]*ocr.PageTranslation
	translateSeq       atomic.Uint64

	// Estado de zoom. zoomFit mide la página al ancho disponible; en modo
	// manual, zoomScale es la escala absoluta (1 = píxel de imagen = unidad Fyne).
	zoomFit   bool
	zoomScale float32
	viewport  fyne.Size
	cleanMode bool
}

func NewGutterApp(a fyne.App) *gutterApp {
	g := &gutterApp{
		singlePage: true,
		zoomFit:    true,
		zoomScale:  1,
		prefs:      a.Preferences(),
	}
	g.recents = loadRecents(g.prefs)
	g.diskCache = newAppDiskCache(a, g.prefs)

	g.window = a.NewWindow(appTitle)
	// Arranque siempre en modo ventana normal. El tamaño definitivo se calcula
	// en OnStarted, cuando Fyne ya conoce el factor de escala real.
	g.window.SetFullScreen(false)
	g.window.Resize(fyne.NewSize(initialWindowWidth, initialWindowHeight))
	g.window.CenterOnScreen()
	// Barras a sangre completa: el padding lo aportan los contenedores internos.
	g.window.SetPadded(false)

	// Icono oficial del sistema (título de ventana + barra de tareas).
	icon := loadBrandResource(iconCandidates...)
	a.SetIcon(icon)
	g.window.SetIcon(icon)

	g.logo = loadBrandResource(logoCandidates...)

	// Controles de lectura que ya no viven en la barra superior (ahora solo
	// lleva el logo y el botón de pantalla completa). Se mantienen como
	// estado para que el menú Ver y la lógica de lectura sigan funcionando.
	g.singleCheck = widget.NewCheck("Modo una página", nil)
	g.singleCheck.SetChecked(g.singlePage)
	g.singleCheck.OnChanged = func(on bool) {
		g.singlePage = on
		g.half = 0
		g.showPage()
		g.unfocus()
	}

	g.direction = widget.NewSelect([]string{readingWestern, readingManga}, nil)
	g.direction.SetSelected(readingWestern)
	g.direction.OnChanged = func(selected string) {
		g.manga = selected == readingManga
		g.half = 0
		g.showPage()
		g.unfocus()
	}

	// Lienzo de página reutilizable. Nunca se sustituye el objeto. El tamaño lo
	// impone pageLayout; FillStretch permite que el zoom sea independiente del
	// tamaño de píxeles original.
	g.pageImage = canvas.NewImageFromImage(nil)
	g.pageImage.FillMode = canvas.ImageFillStretch
	g.pageImage.ScaleMode = canvas.ImageScaleFastest
	g.pageImage.Translucency = 0

	// Capa de traducción: mismo tamaño que la página y transparente fuera de
	// los bocadillos. Se dibuja encima de la imagen original.
	g.translationImage = canvas.NewImageFromImage(nil)
	g.translationImage.FillMode = canvas.ImageFillStretch
	g.translationImage.ScaleMode = canvas.ImageScaleSmooth
	g.translationImage.Hide()
	g.translationCache = make(map[string]*ocr.PageTranslation)

	g.pageLayout = &ui.PageLayout{Width: 1, Height: 1}
	g.pageCanvas = container.NewStack(
		canvas.NewRectangle(theme.GutterCanvasColor()),
		container.New(g.pageLayout, g.pageImage, g.translationImage),
	)

	g.scroll = ui.NewScrollSinSombras(g.pageCanvas)
	g.scroll.Direction = container.ScrollBoth
	g.scroll.SetMinSize(fyne.NewSize(240, 240))
	g.scroll.Hide()

	g.emptyState = container.NewStack()
	g.refreshWelcome()
	g.cursorBlanker = cursor.NewCleanCursor(func() bool { return g.cleanMode })
	g.cursorBlanker.Hide()

	g.pageBadge = newFloatingPill()
	g.pageBadgeOverlay = container.New(
		&topRightLayout{margin: 16, child: g.pageBadge.root},
		g.pageBadge.root,
	)
	g.gotoBadge = newFloatingPill()
	g.gotoBadgeOverlay = container.New(
		&topCenterLayout{margin: 14, child: g.gotoBadge.root},
		g.gotoBadge.root,
	)

	// Marcador de error de página. Es un objeto más del stack de lectura, no un
	// diálogo: así la navegación sigue disponible y el reintento es inmediato.
	g.pageErrorLabel = widget.NewLabelWithStyle(
		"No se pudo mostrar esta página",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	g.pageErrorLabel.Wrapping = fyne.TextWrapWord
	g.pageErrorDetail = widget.NewLabel("")
	g.pageErrorDetail.Wrapping = fyne.TextWrapWord
	g.pageErrorDetail.Importance = widget.LowImportance
	retryButton := widget.NewButtonWithIcon("Reintentar", fyneTheme.ViewRefreshIcon(), g.retryPage)
	retryButton.Importance = widget.HighImportance

	errorCard := container.NewStack(
		canvas.NewRectangle(theme.GutterPanelColor()),
		container.NewPadded(container.NewVBox(
			container.NewCenter(g.pageErrorLabel),
			container.NewCenter(g.pageErrorDetail),
			container.NewCenter(retryButton),
		)),
	)
	g.pageErrorState = container.NewCenter(container.NewGridWrap(fyne.NewSize(440, 230), errorCard))
	g.pageErrorState.Hide()

	readerLayout := &ui.ReaderLayout{OnResize: g.onReaderResize}
	readerArea := container.New(
		readerLayout,
		canvas.NewRectangle(theme.GutterCanvasColor()),
		g.scroll,
		g.emptyState,
		g.cursorBlanker,
	)
	g.bottomShadow = canvas.NewVerticalGradient(color.Transparent, color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99})
	readerStack := container.New(
		&ui.BottomShadowLayout{Height: 2},
		readerArea,
		g.pageErrorState,
		g.pageBadgeOverlay,
		g.gotoBadgeOverlay,
		g.bottomShadow,
	)

	g.bottomBar = g.buildBottomBar()

	g.root = container.New(&ui.RootLayout{}, readerStack, g.bottomBar)
	g.window.SetContent(g.root)

	// Arrastrar y soltar un .cbr/.cbz sobre la ventana.
	g.window.SetOnDropped(g.handleDropped)

	// Guardar progreso y tamaño de ventana antes de destruirla.
	g.window.SetOnClosed(func() {
		g.cancelPageWork()
		g.saveCurrentProgress()
		g.saveWindowSize()
	})

	// Handler de teclado a nivel de ventana. Fyne lo invoca cuando ningún
	// widget con foco ha consumido la tecla.
	g.window.Canvas().SetOnTypedKey(g.handleKey)
	g.window.Canvas().SetOnTypedRune(func(r rune) {
		if r == '?' {
			g.showHelp()
		}
	})
	g.installMenu()

	g.updateControls()
	g.updateZoomLabel()
	return g
}

// fitWindowToPrimaryScreen ajusta el tamaño final usando el área útil real del
// monitor (sin barra de tareas) y vuelve a centrar la ventana. Se ejecuta desde
// OnStarted, cuando Canvas().Scale() ya refleja el DPI efectivo.
func (g *gutterApp) FitWindowToPrimaryScreen() {
	scale := g.window.Canvas().Scale()
	if scale <= 0 {
		scale = 1
	}

	_, _, workW, workH, ok := screen.PrimaryWorkArea()
	if !ok || workW <= 0 || workH <= 0 {
		g.lastWindowedSize = fyne.NewSize(startupWindowWidth, startupWindowHeight)
		g.window.Resize(g.lastWindowedSize)
		g.window.CenterOnScreen()
		return
	}

	// Reserva espacio para la barra de título/bordes del sistema y un margen.
	maxWidth := float32(workW)/scale - windowSideMargin
	maxHeight := float32(workH)/scale - windowChromeHeight
	if maxWidth < 1 {
		maxWidth = 1
	}
	if maxHeight < 1 {
		maxHeight = 1
	}

	// Recupera el último tamaño de ventana, siempre acotado al área útil del
	// monitor actual para que la barra de título no quede fuera de pantalla.
	width := startupWindowWidth
	height := startupWindowHeight
	if saved := g.prefs.Float(prefWindowWidth); saved >= float64(minimumWindowWidth) {
		width = float32(saved)
	}
	if saved := g.prefs.Float(prefWindowHeight); saved >= float64(minimumWindowHeight) {
		height = float32(saved)
	}

	width = fyne.Min(width, maxWidth)
	height = fyne.Min(height, maxHeight)
	if width < minimumWindowWidth {
		width = fyne.Min(minimumWindowWidth, maxWidth)
	}
	if height < minimumWindowHeight {
		height = fyne.Min(minimumWindowHeight, maxHeight)
	}

	g.lastWindowedSize = fyne.NewSize(width, height)
	g.window.Resize(g.lastWindowedSize)
	g.window.CenterOnScreen()
}

// Run muestra la ventana y lanza el bucle de eventos de Fyne.
func (g *gutterApp) Run() {
	g.window.ShowAndRun()
}
