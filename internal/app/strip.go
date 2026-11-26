package app

// strip.go implementa el modo tiras (webtoon): todas las páginas del cómic
// apiladas en vertical, sin separación. Solo se decodifican las páginas cerca
// de la vista; las lejanas liberan su imagen pero conservan su altura, para que
// el desplazamiento no salte al cargar o descargar páginas.
//
// La página actual es la que queda en el borde superior de la vista. Se
// actualiza con OnScrolled, que Fyne llama tanto con la rueda como al arrastrar
// o usar la barra lateral.

import (
	"context"
	"fmt"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/backend"
	gtheme "gutter/internal/theme"
)

const (
	// Proporción alto/ancho supuesta hasta conocer la página real. Es típica de
	// un cómic a página completa.
	stripPlaceholderRatio float32 = 1.5
	// Páginas a cada lado de la vista que se cargan, y distancia a partir de la
	// cual se liberan.
	stripLoadRadius = 2
	stripKeepRadius = 4
)

// stripPage es una página del modo tiras: imagen, marcador de estado y el
// rectángulo de fondo que se ve mientras no hay imagen.
type stripPage struct {
	root        *fyne.Container
	image       *canvas.Image
	placeholder *canvas.Rectangle
	message     *widget.Label
	natW, natH  int
	loading     bool
	failed      bool
}

// stripLayout coloca las páginas una debajo de otra y las centra en horizontal.
type stripLayout struct {
	widths   []float32
	heights  []float32
	tops     []float32
	contentW float32
	contentH float32
}

func (l *stripLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for i, obj := range objects {
		if i >= len(l.tops) {
			break
		}
		w, h := l.widths[i], l.heights[i]
		x := (size.Width - w) / 2
		if x < 0 {
			x = 0
		}
		obj.Resize(fyne.NewSize(w, h))
		obj.Move(fyne.NewPos(x, l.tops[i]))
	}
}

func (l *stripLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.contentW, l.contentH)
}

// stripState es el estado del modo tiras para un cómic concreto.
type stripState struct {
	comic   *backend.Comic
	content *fyne.Container
	layout  *stripLayout
	pages   []*stripPage
	ctx     context.Context
	cancel  context.CancelFunc
}

func (g *gutterApp) buildStrip(comic *backend.Comic) *stripState {
	ctx, cancel := context.WithCancel(g.baseContext())
	s := &stripState{
		comic:  comic,
		layout: &stripLayout{},
		ctx:    ctx,
		cancel: cancel,
	}

	objects := make([]fyne.CanvasObject, 0, len(comic.Pages))
	s.pages = make([]*stripPage, 0, len(comic.Pages))
	for i := range comic.Pages {
		page := &stripPage{
			image:       canvas.NewImageFromImage(nil),
			placeholder: canvas.NewRectangle(gtheme.GutterCanvasColor()),
			message:     widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		}
		page.image.FillMode = canvas.ImageFillStretch
		page.image.ScaleMode = canvas.ImageScaleSmooth
		page.message.SetText(fmt.Sprintf("Página %d", i+1))
		page.message.Importance = widget.LowImportance
		page.root = container.NewStack(page.placeholder, page.image, container.NewCenter(page.message))
		s.pages = append(s.pages, page)
		objects = append(objects, page.root)
	}

	s.content = container.New(s.layout, objects...)
	g.strip = s
	g.recomputeStrip()
	return s
}

// stripViewportWidth devuelve el ancho disponible para las páginas.
func (g *gutterApp) stripViewportWidth() float32 {
	if w := g.reader.viewport.Width; w > 0 {
		return w
	}
	return g.scroll.Size().Width
}

// recomputeStrip recalcula anchos y alturas. Con ajuste al ancho cada página
// ocupa la ventana; con zoom manual se aplica la escala sobre el ancho real.
func (g *gutterApp) recomputeStrip() {
	s := g.strip
	if s == nil {
		return
	}
	viewW := g.stripViewportWidth()
	l := s.layout
	l.widths = make([]float32, len(s.pages))
	l.heights = make([]float32, len(s.pages))
	l.tops = make([]float32, len(s.pages))

	var y, maxW float32
	for i, page := range s.pages {
		w := viewW
		if !g.reader.zoomFit && page.natW > 0 {
			w = float32(page.natW) * g.reader.zoomScale
		}
		h := w * stripPlaceholderRatio
		if page.natW > 0 && page.natH > 0 {
			h = w * float32(page.natH) / float32(page.natW)
		}
		l.widths[i] = w
		l.heights[i] = h
		l.tops[i] = y
		y += h
		if w > maxW {
			maxW = w
		}
	}
	l.contentW = fyne.Max(maxW, viewW)
	l.contentH = y
	s.content.Resize(fyne.NewSize(l.contentW, l.contentH))
	s.content.Refresh()
	g.scroll.Refresh()
}

// indexAt devuelve la página que contiene la coordenada vertical y.
func (s *stripState) indexAt(y float32) int {
	if len(s.layout.tops) == 0 {
		return 0
	}
	i := sort.Search(len(s.layout.tops), func(i int) bool { return s.layout.tops[i] > y }) - 1
	if i < 0 {
		return 0
	}
	if i >= len(s.pages) {
		return len(s.pages) - 1
	}
	return i
}

// enableStrip muestra el modo tiras para el cómic abierto. Reutiliza la
// estructura si ya corresponde al mismo cómic.
func (g *gutterApp) enableStrip() {
	comic := g.reader.comic
	if comic == nil || len(comic.Pages) == 0 {
		return
	}
	g.cancelTranslation()
	g.clearTranslation()
	g.clearPageError()
	g.cancelPageWork()
	g.reader.loading = false
	g.readScanned.Store(0)
	g.readTotal.Store(0)

	if g.strip == nil || g.strip.comic != comic {
		g.stopStrip()
		g.buildStrip(comic)
	}

	if g.scroll.Content != g.strip.content {
		g.scroll.Content = g.strip.content
	}
	g.scroll.OnScrolled = g.onStripScrolled
	g.scroll.Show()
	g.emptyState.Hide()
	g.pageErrorState.Hide()
	g.reader.clampIndex()
	g.recomputeStrip()
	g.scrollStripTo(g.reader.pageIndex, false)

	g.window.SetTitle(fmt.Sprintf("%s — %s", appTitle, comic.Path))
	g.updateControls()
}

// leaveStrip vuelve a la vista de página única: detiene el temporizador, libera
// las cargas y devuelve el lienzo normal al scroll.
func (g *gutterApp) leaveStrip() {
	if g.strip == nil && g.scroll.Content == g.pageCanvas {
		return
	}
	g.stopStrip()
	g.scroll.Content = g.pageCanvas
	g.scroll.Refresh()
}

// stopStrip cancela las cargas y deja de seguir el scroll. No cambia el
// contenido del scroll: para eso está leaveStrip.
func (g *gutterApp) stopStrip() {
	s := g.strip
	if s == nil {
		return
	}
	g.strip = nil
	g.scroll.OnScrolled = nil
	s.cancel()
}

func (g *gutterApp) onStripScrolled(fyne.Position) {
	g.syncStrip()
}

// syncStrip actualiza la página actual según el scroll y carga lo que queda
// cerca de la vista.
func (g *gutterApp) syncStrip() {
	s := g.strip
	if s == nil || !g.reader.webtoon || g.opening {
		return
	}
	index := s.indexAt(g.scroll.Offset.Y)
	if index != g.reader.pageIndex {
		g.reader.pageIndex = index
		g.reader.half = 0
		g.saveCurrentProgress()
		g.updateControls()
	}
	g.loadStripAround(index)
}

// scrollStripTo lleva la vista al inicio de una página. Si save es true, guarda
// el progreso de inmediato.
func (g *gutterApp) scrollStripTo(index int, save bool) {
	s := g.strip
	if s == nil || len(s.pages) == 0 {
		return
	}
	index = clampPage(index, len(s.pages))
	g.reader.pageIndex = index
	g.reader.half = 0
	g.scroll.ScrollToOffset(fyne.NewPos(0, s.layout.tops[index]))
	g.loadStripAround(index)
	if save {
		g.saveCurrentProgress()
	}
	g.updateControls()
}

// stripStep avanza o retrocede una página en el modo tiras.
func (g *gutterApp) stripStep(delta int) {
	if g.strip == nil {
		return
	}
	g.scrollStripTo(g.reader.pageIndex+delta, true)
}

// loadStripAround decodifica las páginas cercanas a index y libera las lejanas.
func (g *gutterApp) loadStripAround(index int) {
	s := g.strip
	if s == nil {
		return
	}
	for i, page := range s.pages {
		distance := i - index
		if distance < 0 {
			distance = -distance
		}
		switch {
		case distance <= stripLoadRadius:
			if page.image.Image == nil && !page.loading && !page.failed {
				g.loadStripPage(s, i)
			}
		case distance > stripKeepRadius && page.image.Image != nil:
			page.image.Image = nil
			page.image.Refresh()
			page.placeholder.Show()
		}
	}
}

func (g *gutterApp) loadStripPage(s *stripState, index int) {
	page := s.pages[index]
	page.loading = true
	page.message.SetText(fmt.Sprintf("Cargando página %d…", index+1))
	page.message.Show()

	name := s.comic.Pages[index]
	go func() {
		img, err := s.comic.PageContext(s.ctx, name)
		fyne.Do(func() {
			if g.strip != s || s.ctx.Err() != nil {
				return
			}
			page.loading = false
			if err != nil {
				page.failed = true
				page.message.SetText(fmt.Sprintf("No se pudo cargar la página %d", index+1))
				page.message.Show()
				return
			}

			// Si una página superior cambia de altura, la vista se corrige para
			// que la página actual no salte.
			current := g.reader.pageIndex
			oldTop := float32(0)
			if current < len(s.layout.tops) {
				oldTop = s.layout.tops[current]
			}

			page.natW = img.Bounds().Dx()
			page.natH = img.Bounds().Dy()
			page.image.Image = img
			page.image.Refresh()
			page.placeholder.Hide()
			page.message.Hide()
			g.recomputeStrip()

			if current < len(s.layout.tops) {
				if delta := s.layout.tops[current] - oldTop; delta != 0 {
					g.scroll.ScrollToOffset(fyne.NewPos(g.scroll.Offset.X, g.scroll.Offset.Y+delta))
				}
			}
		})
	}()
}

// refreshStripColors aplica la paleta activa a los fondos de las páginas.
func (g *gutterApp) refreshStripColors() {
	if g.strip == nil {
		return
	}
	for _, page := range g.strip.pages {
		page.placeholder.FillColor = gtheme.GutterCanvasColor()
		page.placeholder.Refresh()
	}
}

// toggleStrip alterna entre página única y modo tiras, conservando la página.
func (g *gutterApp) toggleStrip() {
	g.reader.webtoon = !g.reader.webtoon
	g.reader.half = 0
	if g.reader.comic == nil {
		g.refreshMenu()
		return
	}
	if g.reader.webtoon {
		g.enableStrip()
	} else {
		g.showPage()
	}
	g.saveCurrentProgress()
	g.refreshMenu()
	g.unfocus()
}

// notifyStripUnavailable avisa cuando una función no existe en modo tiras.
func (g *gutterApp) notifyStripUnavailable(feature string) {
	dialog.ShowInformation(
		"Modo tiras",
		feature+" no está disponible en modo tiras. Desactiva el modo tiras para usarlo.",
		g.window,
	)
}

// hasPage indica si hay contenido que se puede ampliar o ajustar.
func (g *gutterApp) hasPage() bool {
	if g.reader.webtoon {
		return g.strip != nil && len(g.strip.pages) > 0
	}
	return g.pageImage.Image != nil
}
