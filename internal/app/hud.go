// hud.go agrupa los elementos flotantes del lector: el indicador de página
// visible en pantalla completa y el selector rápido de página con G.
package app

import (
	"fmt"
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// floatingPill es una etiqueta sobre un fondo oscuro redondeado. No captura
// clics ni rueda, por lo que no interfiere con el scroll de la página.
//
// Importante: el objeto que se inserta en el árbol de Fyne debe ser el
// *fyne.Container real (campo root), no este struct. El recorrido de pintado
// de Fyne solo desciende a los hijos de contenedores cuyo tipo dinámico es
// exactamente *fyne.Container; un tipo propio que lo embeba se dibujaría como
// una hoja vacía.
type floatingPill struct {
	root  *fyne.Container
	label *widget.Label
}

func newFloatingPill() *floatingPill {
	background := canvas.NewRectangle(color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xE8})
	background.CornerRadius = 8

	label := widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	body := container.New(layout.NewCustomPaddedLayout(4, 4, 10, 10), label)

	pill := &floatingPill{
		root:  container.NewStack(background, body),
		label: label,
	}
	pill.Hide()
	return pill
}

func (p *floatingPill) SetText(text string) {
	p.label.SetText(text)
}

func (p *floatingPill) Show() {
	p.root.Show()
	p.root.Refresh() // Container.Show no marca el lienzo como sucio
}

func (p *floatingPill) Hide() {
	p.root.Hide()
}

func (p *floatingPill) Visible() bool {
	return p.root.Visible()
}

// topRightLayout coloca su hijo en la esquina superior derecha del área de
// lectura, con un margen fijo.
type topRightLayout struct {
	margin float32
	child  fyne.CanvasObject
}

func (l *topRightLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	if l.child == nil || !l.child.Visible() {
		return
	}
	childSize := l.child.MinSize()
	l.child.Resize(childSize)
	x := fyne.Max(l.margin, size.Width-childSize.Width-l.margin)
	l.child.Move(fyne.NewPos(x, l.margin))
}

func (l *topRightLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	if l.child == nil || !l.child.Visible() {
		return fyne.NewSize(1, 1)
	}
	return l.child.MinSize()
}

// topCenterLayout coloca su hijo centrado horizontalmente en la parte superior.
type topCenterLayout struct {
	margin float32
	child  fyne.CanvasObject
}

func (l *topCenterLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	if l.child == nil || !l.child.Visible() {
		return
	}
	childSize := l.child.MinSize()
	l.child.Resize(childSize)
	x := (size.Width - childSize.Width) / 2
	if x < l.margin {
		x = l.margin
	}
	l.child.Move(fyne.NewPos(x, l.margin))
}

func (l *topCenterLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	if l.child == nil || !l.child.Visible() {
		return fyne.NewSize(1, 1)
	}
	return l.child.MinSize()
}

// updatePageBadge refresca el indicador de la esquina. Solo se muestra en el
// modo limpio a pantalla completa, donde las barras están ocultas.
func (g *gutterApp) updatePageBadge() {
	if g.pageBadge == nil {
		return
	}
	if !g.cleanMode || g.comic == nil || len(g.comic.Pages) == 0 {
		g.pageBadge.Hide()
		if g.pageBadgeOverlay != nil {
			g.pageBadgeOverlay.Refresh()
		}
		return
	}

	text := fmt.Sprintf("Página %d / %d", g.pageIndex+1, len(g.comic.Pages))
	g.pageBadge.SetText(text)
	g.pageBadge.Show()
	if g.pageBadgeOverlay != nil {
		g.pageBadgeOverlay.Refresh()
	}
}

// openGotoPage inicia el modo de salto rápido: G, dígitos y Enter.
func (g *gutterApp) openGotoPage() {
	if g.comic == nil || len(g.comic.Pages) == 0 || g.loading {
		return
	}
	g.gotoMode = true
	g.gotoBuffer = ""
	g.gotoError = ""
	g.gotoBadge.Show()
	g.updateGotoBadge()
	g.unfocus()
}

func (g *gutterApp) closeGotoPage() {
	g.gotoMode = false
	g.gotoBuffer = ""
	g.gotoError = ""
	if g.gotoBadge != nil {
		g.gotoBadge.Hide()
		if g.gotoBadgeOverlay != nil {
			g.gotoBadgeOverlay.Refresh()
		}
	}
}

func (g *gutterApp) updateGotoBadge() {
	if g.gotoBadge == nil {
		return
	}

	total := 0
	if g.comic != nil {
		total = len(g.comic.Pages)
	}

	var text string
	switch {
	case g.gotoError != "":
		text = g.gotoError
	case g.gotoBuffer == "":
		text = fmt.Sprintf("G · Ir a página (1–%d)  ·  Enter", total)
	default:
		text = fmt.Sprintf("G · Ir a página: %s  ·  Enter", g.gotoBuffer)
	}
	g.gotoBadge.SetText(text)
	if g.gotoBadgeOverlay != nil {
		g.gotoBadgeOverlay.Refresh()
	}
}

// handleGotoKey procesa las teclas mientras está activo el salto con G.
// Devuelve true si la tecla fue consumida por el selector.
func (g *gutterApp) handleGotoKey(event *fyne.KeyEvent) bool {
	switch event.Name {
	case fyne.KeyEscape:
		g.closeGotoPage()
		return true
	case fyne.KeyBackspace:
		if len(g.gotoBuffer) > 0 {
			g.gotoBuffer = g.gotoBuffer[:len(g.gotoBuffer)-1]
		}
		g.gotoError = ""
		g.updateGotoBadge()
		return true
	case fyne.KeyReturn, fyne.KeyEnter:
		g.commitGotoPage()
		return true
	case fyne.KeyG:
		// Volver a pulsar G reinicia la entrada (útil tras un error).
		g.gotoBuffer = ""
		g.gotoError = ""
		g.updateGotoBadge()
		return true
	}

	if digit, ok := digitKey(event.Name); ok {
		if len(g.gotoBuffer) < 6 {
			g.gotoBuffer += digit
			g.gotoError = ""
			g.updateGotoBadge()
		}
		return true
	}

	// Cualquier otra tecla solo cancela el modo para no disparar acciones
	// (zoom, traducción, navegación) mientras el usuario escribe un número.
	g.closeGotoPage()
	return true
}

func (g *gutterApp) commitGotoPage() {
	if g.gotoBuffer == "" {
		g.closeGotoPage()
		return
	}
	if g.comic == nil {
		g.closeGotoPage()
		return
	}

	page, err := strconv.Atoi(g.gotoBuffer)
	total := len(g.comic.Pages)
	if err != nil || page < 1 || page > total {
		g.gotoBuffer = ""
		g.gotoError = fmt.Sprintf("Página no válida · 1–%d", total)
		g.updateGotoBadge()
		return
	}

	g.closeGotoPage()
	g.goToPage(page - 1)
}

func digitKey(name fyne.KeyName) (string, bool) {
	switch name {
	case fyne.Key0:
		return "0", true
	case fyne.Key1:
		return "1", true
	case fyne.Key2:
		return "2", true
	case fyne.Key3:
		return "3", true
	case fyne.Key4:
		return "4", true
	case fyne.Key5:
		return "5", true
	case fyne.Key6:
		return "6", true
	case fyne.Key7:
		return "7", true
	case fyne.Key8:
		return "8", true
	case fyne.Key9:
		return "9", true
	default:
		return "", false
	}
}
