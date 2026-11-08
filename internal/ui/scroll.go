// scroll.go define el Scroll de lectura sin las sombras internas de Fyne.
package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"image/color"
)

// scrollSinSombras es un Scroll de Fyne sin las sombras de sus bordes.
//
// Fyne crea esas sombras dentro del renderizador interno del Scroll
// (internal/widget/scroller.go) y no ofrece ninguna opción pública para
// desactivarlas. La inferior se dibuja justo encima de la barra de navegación
// y, al ser un degradado casi negro, se percibía como una franja despegada de
// la barra. Aquí se sustituyen en la lista de objetos del renderizador por
// rectángulos transparentes: el degradado desaparece y el área de lectura se
// encuentra con la barra en una línea limpia, igual que en la parte superior.
type ScrollSinSombras struct {
	container.Scroll
}

// newScrollSinSombras crea el Scroll sin sombras y registra el tipo derivado
// como implementación de Base, de modo que el lienzo use este renderizador.
func NewScrollSinSombras(content fyne.CanvasObject) *ScrollSinSombras {
	s := &ScrollSinSombras{}
	s.ExtendBaseWidget(s)
	s.Content = content
	return s
}

// CreateRenderer envuelve el renderizador original y descarta sus sombras.
func (s *ScrollSinSombras) CreateRenderer() fyne.WidgetRenderer {
	r := s.Scroll.CreateRenderer()
	objects := r.Objects()
	for i, obj := range objects {
		// Las sombras son del tipo interno *widget.Shadow. No se puede
		// importar ese paquete, así que se identifican por su nombre de tipo.
		if strings.HasSuffix(fmt.Sprintf("%T", obj), ".Shadow") {
			objects[i] = canvas.NewRectangle(color.Transparent)
		}
	}
	return r
}
