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

	view := g.reader.viewport
	if view.Width <= 0 || view.Height <= 0 {
		view = g.scroll.Size()
	}
	if view.Width <= 0 || view.Height <= 0 {
		return
	}

	scale := g.reader.zoomScale
	if g.reader.zoomFit {
		avail := view.Width - 2*pageMargin
		if avail < 80 {
			avail = view.Width
		}
		scale = (avail / float32(b.Dx())) * defaultZoomFactor
	}
	scale = clampZoom(scale)
	g.reader.setZoomScale(scale)

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
	if g.reader.viewport.Height > 0 {
		return fyne.Max(60, g.reader.viewport.Height*0.10)
	}
	return 80
}

// zoomBy cambia a zoom manual manteniendo el nivel actual como punto de partida.

func (g *gutterApp) zoomBy(factor float32) {
	if g.reader.comic == nil || g.pageImage.Image == nil || g.reader.loading {
		return
	}
	g.reader.zoomBy(factor)
	g.applyPageZoom(true)
	g.unfocus()
}

// fitWidth vuelve al modo de ajuste al ancho del área de lectura.

func (g *gutterApp) fitWidth() {
	if g.pageImage.Image == nil {
		return
	}
	g.reader.fitWidth()
	g.applyPageZoom(true)
	g.unfocus()
}

// zoom100 muestra la página a escala 1:1 (un píxel de imagen = una unidad Fyne).

func (g *gutterApp) zoom100() {
	if g.pageImage.Image == nil {
		return
	}
	g.reader.zoom100()
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
	if !g.reader.next() {
		return
	}
	g.showPage()
}

func (g *gutterApp) previousPage() {
	defer g.unfocus()

	if !g.reader.previous() {
		return
	}
	g.showPage()
}

func (g *gutterApp) goToPage(index int) {
	defer g.unfocus()

	if g.reader.comic == nil {
		return
	}
	g.reader.goTo(index)
	g.showPage()
}

// prefetchAdjacent precarga las dos páginas siguientes y la anterior. En un
// RAR sólido el backend recorre el contenedor una sola vez para todas ellas.
// La profundidad se mantiene corta a propósito: con el presupuesto de 64 MiB,
// precargar más páginas expulsaría las que el lector acaba de mostrar.
func (g *gutterApp) prefetchAdjacent(ctx context.Context, comic *backend.Comic) {
	if comic == nil || ctx == nil || g.reader.comic != comic {
		return
	}

	names := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	appendPage := func(index int) {
		if index < 0 || index >= len(comic.Pages) || index == g.reader.pageIndex {
			return
		}
		name := comic.Pages[index]
		if _, duplicate := seen[name]; duplicate {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	for offset := 1; offset <= 2; offset++ {
		appendPage(g.reader.pageIndex + offset)
	}
	appendPage(g.reader.pageIndex - 1)

	if len(names) == 0 {
		return
	}

	go func() {
		_ = comic.PrefetchContext(ctx, names)
	}()
}

// updateControls sincroniza barra inferior y botones con el estado actual.
func (g *gutterApp) updateControls() {
	// El indicador de página de pantalla completa debe seguir a la navegación,
	// no solo aparecer al entrar en modo limpio.
	defer g.updatePageBadge()

	if g.opening {
		g.progress.SetText("Abriendo cómic… · Esc para cancelar")
		g.prevButton.Disable()
		g.nextButton.Disable()
		g.updateZoomLabel()
		return
	}

	if g.reader.comic == nil {
		g.progress.SetText("Sin cómic abierto")
		g.prevButton.Disable()
		g.nextButton.Disable()
		g.updateZoomLabel()
		return
	}

	total := len(g.reader.comic.Pages)
	if g.reader.pageIndex < 0 || g.reader.pageIndex >= total {
		g.reader.pageIndex = 0
	}

	if g.reader.loading {
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
				g.reader.pageIndex+1, total,
			))
		}
		g.prevButton.Disable()
		g.nextButton.Disable()
		g.updateZoomLabel()
		return
	}

	progress := fmt.Sprintf("Página %d de %d", g.reader.pageIndex+1, total)
	if g.pageFailed {
		progress += "  ·  error"
	} else if g.reader.singlePage && g.reader.spread {
		progress += fmt.Sprintf("  ·  %d/2", g.reader.half+1)
	}
	g.progress.SetText(progress)

	toggleEnabled(g.prevButton, g.reader.canGoBack())
	toggleEnabled(g.nextButton, g.reader.canGoForward())

	g.updateZoomLabel()
}

// updateZoomLabel refresca el indicador de porcentaje de zoom.
func (g *gutterApp) updateZoomLabel() {
	if g.reader.zoomFit {
		g.zoomLabel.SetText("Ajuste")
	} else {
		pct := int(g.reader.zoomScale * 100)
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
