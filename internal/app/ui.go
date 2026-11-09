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
	gtheme "gutter/internal/theme"
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

	barBg := canvas.NewRectangle(gtheme.GutterPanelColor())
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
	if size.Width <= 0 || size.Height <= 0 || size == g.viewport {
		return
	}
	g.viewport = size
	if g.zoomFit && g.pageImage.Image != nil {
		g.applyPageZoom(false)
	}
}
func (g *gutterApp) toggleCleanMode() {
	if !g.cleanMode {
		g.cleanMode = true
		g.bottomBar.Hide()
		// Archivo/Ver/Traducción/Ayuda sobre la página.
		if g.mainMenu != nil {
			g.window.SetMainMenu(nil)
		}
		g.cursorBlanker.Show()
		g.window.SetFullScreen(true)
	} else {
		g.cleanMode = false
		g.bottomBar.Show()
		if g.mainMenu != nil {
			g.window.SetMainMenu(g.mainMenu)
		}
		g.cursorBlanker.Hide()
		g.window.SetFullScreen(false)
	}
	g.root.Refresh()
	g.updatePageBadge()
	g.unfocus()
}

// showPage carga la página y la mitad actuales sin tocar el árbol de widgets.
func (g *gutterApp) showPage() {
	if g.comic == nil {
		return
	}
	// Invalida cualquier traducción en vuelo y retira la capa anterior.
	g.translateSeq.Add(1)
	g.translating = false
	g.clearTranslation()
	g.clearPageError()
	if g.pageImage.Image != nil {
		g.scroll.Show()
	}

	if g.pageIndex < 0 {
		g.pageIndex = 0
	}
	if g.pageIndex >= len(g.comic.Pages) {
		g.pageIndex = len(g.comic.Pages) - 1
	}

	comic := g.comic
	name := comic.Pages[g.pageIndex]
	single := g.singlePage
	manga := g.manga
	half := g.half
	seq := g.pageSeq.Add(1)

	// Cancela la página/prefetch anteriores antes de empezar. Así un cambio
	// rápido de página no deja goroutines leyendo y descomprimiendo de fondo.
	g.cancelPageWork()
	g.readScanned.Store(0)
	g.readTotal.Store(0)

	pageCtx, pageCancel := context.WithCancel(context.Background())
	g.pageCancel = pageCancel
	pageCtx = backend.WithReadProgress(pageCtx, func(scanned, total int) {
		g.readScanned.Store(int64(scanned))
		g.readTotal.Store(int64(total))
		fyne.Do(func() {
			if seq == g.pageSeq.Load() && g.loading {
				g.updateControls()
			}
		})
	})

	// El prefetch tiene su propio contexto: se cancela al cambiar de página,
	// pero no arrastra el callback de progreso de la página actual.
	prefetchCtx, prefetchCancel := context.WithCancel(context.Background())
	g.prefetchCancel = prefetchCancel

	g.loading = true
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

		fyne.Do(func() {
			if seq != g.pageSeq.Load() {
				return // llegó tarde: el usuario ya cambió de página
			}

			g.loading = false
			g.pageFailed = false
			g.spread = spread
			g.half = half
			g.shownComic = comic
			g.shownPage = g.pageIndex
			g.shownHalf = half
			g.shownSpread = spread
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
	g.window.SetTitle(fmt.Sprintf("%s — %s", appTitle, filepath.Base(g.comic.Path)))

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
		g.loading = false
		g.readScanned.Store(0)
		g.readTotal.Store(0)
		g.pageFailed = true
		g.spread = false
		g.half = 0

		total := 0
		if g.comic != nil {
			total = len(g.comic.Pages)
		}
		if g.pageIndex >= 0 && total > 0 {
			g.pageErrorLabel.SetText(fmt.Sprintf(
				"No se pudo mostrar la página %d de %d",
				g.pageIndex+1, total,
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

// reportOpenError mantiene el diálogo para los fallos al abrir un archivo:
// en ese caso no hay una página concreta que sustituir por un marcador.
func (g *gutterApp) reportOpenError(seq uint64, err error) {
	if err == nil {
		return
	}
	fyne.Do(func() {
		if seq != g.pageSeq.Load() {
			return
		}
		g.loading = false
		g.clearPageError()
		g.updateControls()
		dialog.ShowError(fmt.Errorf("No se pudo abrir el cómic.\n\n%w", err), g.window)
	})
}

func (g *gutterApp) clearPageError() {
	g.pageFailed = false
	if g.pageErrorState != nil {
		g.pageErrorState.Hide()
	}
}

func (g *gutterApp) retryPage() {
	if g.comic == nil {
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
	if g.gotoMode && g.handleGotoKey(event) {
		return
	}

	switch event.Name {
	case fyne.KeyRight:
		if g.manga {
			g.previousPage()
		} else {
			g.nextPage()
		}
	case fyne.KeyLeft:
		if g.manga {
			g.nextPage()
		} else {
			g.previousPage()
		}
	case fyne.KeyHome:
		g.goToPage(0)
	case fyne.KeyEnd:
		if g.comic != nil {
			g.goToPage(len(g.comic.Pages) - 1)
		}
	case fyne.KeyG:
		g.openGotoPage()
	case fyne.KeyUp:
		g.scrollVertical(g.scrollStep())
	case fyne.KeyDown:
		g.scrollVertical(-g.scrollStep())
	case fyne.KeyPlus, fyne.KeyEqual:
		g.zoomBy(zoomStep)
	case fyne.KeyMinus:
		g.zoomBy(1 / zoomStep)
	case fyne.Key0:
		g.fitWidth()
	case fyne.Key1:
		g.zoom100()
	case fyne.KeyF11:
		g.toggleCleanMode()
	case fyne.KeyF1:
		g.showHelp()
	case fyne.KeyT:
		g.toggleTranslation()
	case fyne.KeyEscape:
		if g.loading {
			g.cancelLoading()
			return
		}
		if g.cleanMode {
			g.toggleCleanMode()
		}
	}
}

// unfocus devuelve el foco al canvas para que las flechas sigan funcionando
// después de pulsar un botón, un checkbox o un selector.
func (g *gutterApp) unfocus() {
	g.window.Canvas().Unfocus()
}

// syncReadingControls refleja en la barra superior el estado restaurado de un
// cómic. El flag evita que SetChecked/SetSelected disparen showPage.
func (g *gutterApp) syncReadingControls() {
	if g.singleCheck == nil || g.direction == nil {
		return
	}
	g.syncingControls = true
	g.singleCheck.SetChecked(g.singlePage)
	if g.manga {
		g.direction.SetSelected(readingManga)
	} else {
		g.direction.SetSelected(readingWestern)
	}
	g.syncingControls = false
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
