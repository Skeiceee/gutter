// ui.go construye la interfaz de Gutter y actúa como controlador entre el
// backend de lectura y los widgets de Fyne.
//
// Regla de oro de rendimiento: el canvas.Image se crea UNA sola vez. Al cambiar
// de página solo se reasigna su campo Image y se llama a Refresh(), evitando
// reconstruir el árbol de widgets, el overlay o el área de lectura.
package app

import (
	"context"
	"embed"
	"fmt"
	"image/color"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/backend"
	"gutter/internal/ui"
)

// Los SVG viajan dentro del binario como respaldo. Si la carpeta logos/ existe
// junto al ejecutable, se lee primero desde disco para permitir personalizar la
// marca sin recompilar.
//
//go:embed logos/*.svg
var brandFiles embed.FS

const (
	appTitle = "Gutter"

	readingWestern = "Occidental (I→D)"
	readingManga   = "Manga (D→I)"

	pageMargin float32 = 12
	minZoom    float32 = 0.05
	maxZoom    float32 = 6
	zoomStep   float32 = 1.25

	// Por defecto la página ocupa el 65 % del ancho disponible: deja aire a los
	// lados y sigue siendo legible sin llegar a ocupar toda la ventana.
	defaultZoomFactor float32 = 0.65

	// Geometría de arranque. El tamaño "inicial" es deliberadamente conservador
	// y se ajusta después a la pantalla real desde fitWindowToPrimaryScreen.
	startupWindowWidth  float32 = 900
	startupWindowHeight float32 = 800
	initialWindowWidth  float32 = 760
	initialWindowHeight float32 = 600

	windowSideMargin    float32 = 40
	windowChromeHeight  float32 = 80
	minimumWindowWidth  float32 = 600
	minimumWindowHeight float32 = 400
)

// pageLayout centra la página dentro del área de lectura, tanto si es mayor
// como si es más pequeña que el viewport. El Scroll usa MinSize() para
// decidir si debe mostrar barras de desplazamiento.
var (
	// Ruta canónica solicitada; los demás son respaldos reales del repositorio.
	logoCandidates = []string{
		"logos/gutter_logo.svg",
		"logos/gutter_logo_principal_blanco_accent.svg",
		"logos/gutter_logo_principal_blanco.svg",
		"logos/gutter_logo_principal.svg",
	}
	// Para el icono del sistema se prefiere la versión cuadrada: un wordmark
	// horizontal se vuelve ilegible en la barra de tareas.
	iconCandidates = []string{
		"logos/gutter_logo.svg",
		"logos/gutter_icono_blanco_accent.svg",
		"logos/gutter_icono.svg",
		"logos/gutter_simbolo_blanco_accent.svg",
		"logos/gutter_logo_principal_blanco_accent.svg",
	}
)

// bottomBarLayout coloca la navegación y centra verticalmente cada widget de
// forma explícita. Evita que los botones se estiren a toda la altura.
func (g *gutterApp) buildBottomBar() fyne.CanvasObject {
	g.prevButton = widget.NewButtonWithIcon("Anterior", theme.NavigateBackIcon(), g.previousPage)
	g.nextButton = widget.NewButtonWithIcon("Siguiente", theme.NavigateNextIcon(), g.nextPage)

	g.progress = widget.NewLabelWithStyle(
		"Sin cómic abierto",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	g.progress.Truncation = fyne.TextTruncateEllipsis
	g.progress.Wrapping = fyne.TextWrapOff

	g.zoomLabel = widget.NewLabelWithStyle("Ancho", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	zoomLabelBox := container.NewGridWrap(fyne.NewSize(56, 36), g.zoomLabel)

	zoomOut := widget.NewButtonWithIcon("", theme.ZoomOutIcon(), func() { g.zoomBy(1 / zoomStep) })
	zoomOut.Importance = widget.LowImportance
	zoomIn := widget.NewButtonWithIcon("", theme.ZoomInIcon(), func() { g.zoomBy(zoomStep) })
	zoomIn.Importance = widget.LowImportance
	fitButton := widget.NewButtonWithIcon("", theme.ZoomFitIcon(), g.fitWidth)
	fitButton.Importance = widget.LowImportance

	zoomControls := container.New(
		layout.NewCustomPaddedHBoxLayout(2),
		zoomOut, zoomLabelBox, zoomIn, fitButton,
	)

	center := container.New(
		&ui.BottomCenterLayout{Progress: g.progress, Zoom: zoomControls},
		g.progress,
		zoomControls,
	)

	barLayout := &ui.BottomBarLayout{
		HPad:   6,
		VPad:   4,
		Prev:   g.prevButton,
		Center: center,
		Next:   g.nextButton,
		Zoom:   zoomControls,
	}

	barBg := g.themedRect(panelTint)
	barBg.Shadow.Color = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99}
	barBg.Shadow.BlurRadius = 2
	barBg.Shadow.Offset = fyne.NewPos(0, -1)
	barBg.Shadow.Spread = 0
	barBg.Shadow.Variant = canvas.DropShadow

	return container.NewStack(
		barBg,
		container.New(barLayout, g.prevButton, center, g.nextButton),
	)
}

// onReaderResize se invoca cuando cambia el área central (ventana, barras
// ocultas, etc.). Mantiene el ajuste al ancho sincronizado con el viewport.
func (g *gutterApp) onReaderResize(size fyne.Size) {
	if !g.reader.setViewport(size) {
		return
	}
	if g.reader.zoomFit && g.hasPage() {
		g.applyPageZoom(false)
	}
}

func (g *gutterApp) toggleCleanMode() {
	if !g.reader.cleanMode {
		g.reader.cleanMode = true
		g.bottomBar.Hide()
		// Archivo/Ver/Traducción/Ayuda sobre la página.
		if g.mainMenu != nil {
			g.window.SetMainMenu(nil)
		}
		g.cursorBlanker.Show()
		g.window.SetFullScreen(true)
	} else {
		g.reader.cleanMode = false
		g.bottomBar.Show()
		// Se reconstruye para que las marcas del menú Ver reflejen el estado
		// actual aunque se hayan cambiado las preferencias en modo limpio.
		g.installMenu()
		g.cursorBlanker.Hide()
		g.window.SetFullScreen(false)
	}
	g.root.Refresh()
	g.updatePageBadge()
	g.unfocus()
}

// showPage carga la página y la mitad actuales sin tocar el árbol de widgets.
func (g *gutterApp) showPage() {
	if g.reader.comic == nil {
		return
	}
	// Mientras se indexa otro cómic, la vista sigue mostrando el actual: no se
	// lanza una carga nueva que nacería obsoleta.
	if g.opening {
		return
	}
	// Cancela cualquier traducción en vuelo (OCR y red) y retira la capa
	// anterior: ya no corresponde a la página que se va a mostrar.
	g.cancelTranslation()
	g.clearTranslation()
	if g.reader.webtoon {
		g.enableStrip()
		return
	}
	g.leaveStrip()
	g.clearPageError()
	if g.pageImage.Image != nil {
		g.scroll.Show()
	}

	g.reader.clampIndex()

	comic := g.reader.comic
	name := g.reader.pageName()
	single := g.reader.singlePage
	manga := g.reader.manga
	half := g.reader.half
	seq := g.pageSeq.Add(1)

	// Cancela la página/prefetch anteriores antes de empezar. Así un cambio
	// rápido de página no deja goroutines leyendo y descomprimiendo de fondo.
	g.cancelPageWork()
	g.readScanned.Store(0)
	g.readTotal.Store(0)

	pageCtx, pageCancel := context.WithCancel(g.baseContext())
	g.pageCancel = pageCancel
	pageCtx = backend.WithReadProgress(pageCtx, func(scanned, total int) {
		g.readScanned.Store(int64(scanned))
		g.readTotal.Store(int64(total))
		fyne.Do(func() {
			if seq == g.pageSeq.Load() && g.reader.loading {
				g.updateControls()
			}
		})
	})

	// El prefetch tiene su propio contexto: se cancela al cambiar de página,
	// pero no arrastra el callback de progreso de la página actual.
	prefetchCtx, prefetchCancel := context.WithCancel(g.baseContext())
	g.prefetchCancel = prefetchCancel

	g.reader.loading = true
	g.updateControls()

	go func() {
		spread, err := comic.IsSpreadContext(pageCtx, name)
		if err != nil {
			g.reportLoadError(seq, pageCtx, err)
			return
		}

		// Una página vertical no se parte aunque el modo una página esté activo.
		if !single || !spread {
			half = 0
		}
		rightHalf := isRightHalf(half, manga)

		img, err := comic.GetPageContext(pageCtx, name, single, rightHalf)
		if err != nil {
			g.reportLoadError(seq, pageCtx, err)
			return
		}

		// Solo la página mostrada se guarda en disco. El prefetch se queda en
		// RAM: persistirlo multiplicaba la escritura sin mejorar la lectura.
		if seq == g.pageSeq.Load() {
			comic.PersistPage(name)
		}

		fyne.Do(func() {
			if seq != g.pageSeq.Load() {
				return // llegó tarde: el usuario ya cambió de página
			}

			g.pageFailed = false
			g.reader.markShown(spread, half)
			g.readScanned.Store(0)
			g.readTotal.Store(0)
			g.pageErrorState.Hide()

			// Optimización crítica: reutilizamos el canvas.Image existente.
			g.pageImage.Image = img
			g.pageImage.Refresh()

			g.scroll.Show()
			g.emptyState.Hide()
			g.applyPageZoom(true)
			g.updateControls()
			g.saveCurrentProgress()
			g.prefetchAdjacent(prefetchCtx, comic)
		})
	}()
	g.window.SetTitle(fmt.Sprintf("%s — %s", appTitle, filepath.Base(g.reader.comic.Path)))

	// updateControls centraliza el texto de progreso (incluye el marcador de
	// error) y el estado de los botones.
	g.updateControls()
}

// reportLoadError muestra un marcador dentro del área de lectura. No abre un
// diálogo modal: el usuario puede seguir navegando o reintentar esa página.
func (g *gutterApp) reportLoadError(seq uint64, ctx context.Context, err error) {
	if ctx != nil && ctx.Err() != nil {
		return
	}
	fyne.Do(func() {
		if seq != g.pageSeq.Load() {
			return
		}
		g.reader.loading = false
		g.readScanned.Store(0)
		g.readTotal.Store(0)
		g.pageFailed = true
		g.reader.spread = false
		g.reader.half = 0

		total := g.reader.total()
		if g.reader.pageIndex >= 0 && total > 0 {
			g.pageErrorLabel.SetText(fmt.Sprintf(
				"No se pudo mostrar la página %d de %d",
				g.reader.pageIndex+1, total,
			))
		} else {
			g.pageErrorLabel.SetText("No se pudo mostrar la página")
		}
		g.pageErrorDetail.SetText(err.Error())
		g.pageErrorState.Show()
		g.scroll.Hide()
		g.emptyState.Hide()
		g.updateControls()
	})
}

func (g *gutterApp) clearPageError() {
	g.pageFailed = false
	if g.pageErrorState != nil {
		g.pageErrorState.Hide()
	}
}

func (g *gutterApp) retryPage() {
	if g.reader.comic == nil {
		return
	}
	g.showPage()
}

// toggleEnabled evita refrescos de widget innecesarios en cada página.
func toggleEnabled(button *widget.Button, enabled bool) {
	if enabled == !button.Disabled() {
		return
	}
	if enabled {
		button.Enable()
	} else {
		button.Disable()
	}
}

// handleKey implementa los atajos globales. En Manga se invierten las flechas.
// Arriba/Abajo desplazan el scroll; +/− ajustan el zoom, 0 vuelve a ajuste al
// ancho, 1 aplica 100 % y F11/Esc alternan el modo de lectura limpia.
func (g *gutterApp) handleKey(event *fyne.KeyEvent) {
	if g.gridOpen {
		g.handleGridKey(event)
		return
	}
	if g.gotoMode && g.handleGotoKey(event) {
		return
	}

	switch event.Name {
	case fyne.KeyEscape:
		if g.reader.loading || g.opening {
			g.cancelLoading()
			return
		}
		if g.reader.cleanMode {
			g.toggleCleanMode()
		}
		return
	case fyne.KeyF1:
		g.showHelp()
		return
	}

	if action, ok := g.actionFor(event.Name); ok {
		g.runAction(action)
	}
}

// runAction ejecuta la función de una acción de lectura. Las flechas de página
// se invierten en manga, por eso se resuelven aquí y no en el mapa de teclas.
func (g *gutterApp) runAction(action shortcutAction) {
	switch action {
	case actNextPage:
		if g.reader.manga {
			g.previousPage()
		} else {
			g.nextPage()
		}
	case actPrevPage:
		if g.reader.manga {
			g.nextPage()
		} else {
			g.previousPage()
		}
	case actFirstPage:
		g.goToPage(0)
	case actLastPage:
		if g.reader.comic != nil {
			g.goToPage(len(g.reader.comic.Pages) - 1)
		}
	case actGotoPage:
		g.openGotoPage()
	case actThumbGrid:
		g.toggleThumbnailGrid()
	case actScrollUp:
		g.scrollVertical(g.scrollStep())
	case actScrollDown:
		g.scrollVertical(-g.scrollStep())
	case actZoomIn:
		g.zoomBy(zoomStep)
	case actZoomOut:
		g.zoomBy(1 / zoomStep)
	case actFitWidth:
		g.fitWidth()
	case actZoom100:
		g.zoom100()
	case actCleanMode:
		g.toggleCleanMode()
	case actWebtoon:
		g.toggleStrip()
	case actTranslate:
		g.toggleTranslation()
	case actBookmark:
		g.openBookmarkDialog()
	}
}

// unfocus devuelve el foco al canvas para que las flechas sigan funcionando
// después de pulsar un botón, un checkbox o un selector.
func (g *gutterApp) unfocus() {
	g.window.Canvas().Unfocus()
}

// setSinglePage cambia entre página completa y mitades, guarda el progreso y
// refresca el menú para que la marca de verificación siga el estado real.
func (g *gutterApp) setSinglePage(on bool) {
	if g.reader.singlePage == on {
		return
	}
	g.reader.singlePage = on
	g.reader.half = 0
	g.showPage()
	g.saveCurrentProgress()
	g.refreshMenu()
	g.unfocus()
}

// setReadingDirection cambia el sentido de lectura (occidental o manga).
func (g *gutterApp) setReadingDirection(manga bool) {
	if g.reader.manga == manga {
		return
	}
	g.reader.manga = manga
	g.reader.half = 0
	g.showPage()
	g.saveCurrentProgress()
	g.refreshMenu()
	g.unfocus()
}

// handleDropped abre el primer .cbr/.cbz soltado sobre la ventana.
func (g *gutterApp) handleDropped(_ fyne.Position, uris []fyne.URI) {
	for _, uri := range uris {
		if uri == nil || uri.Scheme() != "file" {
			continue
		}
		if !isComicPath(uri.Path()) {
			continue
		}
		g.LoadComic(uri.Path())
		return
	}
	dialog.ShowError(
		fmt.Errorf("Suelta un archivo .cbr o .cbz sobre la ventana."),
		g.window,
	)
}
