package app

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"gutter/internal/backend"
)

func (g *gutterApp) applyPageZoom(resetScroll bool) {
	img := g.pageImage.Image
	if img == nil {
		return
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return
	}

	view := g.viewport
	if view.Width <= 0 || view.Height <= 0 {
		view = g.scroll.Size()
	}
	if view.Width <= 0 || view.Height <= 0 {
		return
	}

	scale := g.zoomScale
	if g.zoomFit {
		avail := view.Width - 2*pageMargin
		if avail < 80 {
			avail = view.Width
		}
		scale = (avail / float32(b.Dx())) * defaultZoomFactor
	}
	scale = clampZoom(scale)
	g.zoomScale = scale

	w := float32(b.Dx()) * scale
	h := float32(b.Dy()) * scale

	g.pageLayout.Width = w
	g.pageLayout.Height = h
	g.pageCanvas.Resize(fyne.NewSize(w, h))
	g.pageCanvas.Refresh()
	g.scroll.Refresh()

	if resetScroll {
		g.scroll.ScrollToOffset(fyne.NewPos(0, 0))
	} else {
		// Al redimensionar, evita conservar un offset que ya no existe.
		off := g.scroll.Offset
		maxX := fyne.Max(0, w-view.Width)
		maxY := fyne.Max(0, h-view.Height)
		if off.X > maxX {
			off.X = maxX
		}
		if off.Y > maxY {
			off.Y = maxY
		}
		if off != g.scroll.Offset {
			g.scroll.ScrollToOffset(off)
		}
	}
	g.updateZoomLabel()
}

// scrollVertical desplaza el contenido del cómic. DY positivo mueve hacia el
// inicio de la página; negativo, hacia el final.

func (g *gutterApp) scrollVertical(dy float32) {
	if g.scroll == nil || g.scroll.Content == nil {
		return
	}
	g.scroll.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.NewDelta(0, dy)})
}

func (g *gutterApp) scrollStep() float32 {
	if g.viewport.Height > 0 {
		return fyne.Max(60, g.viewport.Height*0.10)
	}
	return 80
}

// zoomBy cambia a zoom manual manteniendo el nivel actual como punto de partida.

func (g *gutterApp) zoomBy(factor float32) {
	if g.comic == nil || g.pageImage.Image == nil || g.loading {
		return
	}
	g.zoomFit = false
	g.zoomScale = clampZoom(g.zoomScale * factor)
	g.applyPageZoom(true)
	g.unfocus()
}

// fitWidth vuelve al modo de ajuste al ancho del área de lectura.

func (g *gutterApp) fitWidth() {
	if g.pageImage.Image == nil {
		return
	}
	g.zoomFit = true
	g.applyPageZoom(true)
	g.unfocus()
}

// zoom100 muestra la página a escala 1:1 (un píxel de imagen = una unidad Fyne).

func (g *gutterApp) zoom100() {
	if g.pageImage.Image == nil {
		return
	}
	g.zoomFit = false
	g.zoomScale = 1
	g.applyPageZoom(true)
	g.unfocus()
}

// toggleCleanMode oculta las barras y pasa a pantalla completa. F11 o Esc
// restauran la interfaz.

func isRightHalf(half int, manga bool) bool {
	second := half%2 == 1
	if manga {
		return !second
	}
	return second
}

func (g *gutterApp) nextPage() {
	defer g.unfocus()

	// Se permite navegar durante una carga: showPage cancela la anterior. Así
	// una pulsación rápida de flecha no espera a que termine un RAR sólido.
	if g.comic == nil {
		return
	}

	// Mientras carga, no conocemos todavía si la página destino es un spread:
	// avanzamos de página directamente y dejamos que showPage lo resuelva.
	if g.loading {
		if g.pageIndex < len(g.comic.Pages)-1 {
			g.pageIndex++
			g.half = 0
			g.showPage()
		}
		return
	}

	// Primero la segunda mitad de una doble página.
	if g.singlePage && g.spread && g.half == 0 {
		g.half = 1
		g.showPage()
		return
	}

	if g.pageIndex < len(g.comic.Pages)-1 {
		g.pageIndex++
		g.half = 0
		g.showPage()
	}
}

func (g *gutterApp) previousPage() {
	defer g.unfocus()

	if g.comic == nil {
		return
	}

	if g.loading {
		if g.pageIndex > 0 {
			g.pageIndex--
			g.half = 0
			g.showPage()
		}
		return
	}

	if g.singlePage && g.spread && g.half == 1 {
		g.half = 0
		g.showPage()
		return
	}

	if g.pageIndex > 0 {
		g.pageIndex--
		// Al retroceder, si la página anterior era doble, mostramos su última
		// mitad. showPage lo corrige a 0 si finalmente no es una doble página.
		g.half = 1
		g.showPage()
	}
}

func (g *gutterApp) goToPage(index int) {
	defer g.unfocus()

	if g.comic == nil {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(g.comic.Pages) {
		index = len(g.comic.Pages) - 1
	}
	g.pageIndex = index
	g.half = 0
	g.showPage()
}

// prefetchAdjacent precarga las tres páginas siguientes y la anterior. En un
// RAR sólido el backend recorre el contenedor una sola vez para todas ellas.
func (g *gutterApp) prefetchAdjacent(ctx context.Context, comic *backend.Comic) {
	if comic == nil || ctx == nil || g.comic != comic {
		return
	}

	names := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	appendPage := func(index int) {
		if index < 0 || index >= len(comic.Pages) || index == g.pageIndex {
			return
		}
		name := comic.Pages[index]
		if _, duplicate := seen[name]; duplicate {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	for offset := 1; offset <= 3; offset++ {
		appendPage(g.pageIndex + offset)
	}
	appendPage(g.pageIndex - 1)

	if len(names) == 0 {
		return
	}

	go func() {
		_ = comic.PrefetchContext(ctx, names)
	}()
}

// updateControls sincroniza barra inferior y botones con el estado actual.
func (g *gutterApp) updateControls() {
	if g.comic == nil {
		g.progress.SetText("Sin cómic abierto")
		g.prevButton.Disable()
		g.nextButton.Disable()
		g.updateZoomLabel()
		return
	}

	total := len(g.comic.Pages)
	if g.pageIndex < 0 || g.pageIndex >= total {
		g.pageIndex = 0
	}

	if g.loading {
		scanned := int(g.readScanned.Load())
		readTotal := int(g.readTotal.Load())
		if scanned > 0 && readTotal > 0 {
			g.progress.SetText(fmt.Sprintf(
				"RAR sólido %d/%d · Esc para cancelar",
				scanned, readTotal,
			))
		} else {
			g.progress.SetText(fmt.Sprintf(
				"Cargando página %d de %d… · Esc para cancelar",
				g.pageIndex+1, total,
			))
		}
		g.prevButton.Disable()
		g.nextButton.Disable()
		g.updateZoomLabel()
		return
	}

	progress := fmt.Sprintf("Página %d de %d", g.pageIndex+1, total)
	if g.pageFailed {
		progress += "  ·  error"
	} else if g.singlePage && g.spread {
		progress += fmt.Sprintf("  ·  %d/2", g.half+1)
	}
	g.progress.SetText(progress)

	canPrev := g.pageIndex > 0 || (g.singlePage && g.spread && g.half == 1)
	canNext := g.pageIndex < total-1 || (g.singlePage && g.spread && g.half == 0)
	toggleEnabled(g.prevButton, canPrev)
	toggleEnabled(g.nextButton, canNext)

	g.updateZoomLabel()
}

// updateZoomLabel refresca el indicador de porcentaje de zoom.
func (g *gutterApp) updateZoomLabel() {
	if g.zoomFit {
		g.zoomLabel.SetText("Ajuste")
	} else {
		pct := int(g.zoomScale * 100)
		g.zoomLabel.SetText(fmt.Sprintf("%d %%", pct))
	}
}

// clampZoom limita el factor de zoom al rango permitido.
func clampZoom(scale float32) float32 {
	if scale < minZoom {
		return minZoom
	}
	if scale > maxZoom {
		return maxZoom
	}
	return scale
}
