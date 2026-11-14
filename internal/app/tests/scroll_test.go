package tests

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"gutter/internal/ui"
)

// contarSombras devuelve cuántos objetos del renderizador son sombras
// internas de Fyne (*widget.Shadow).
func contarSombras(objetos []fyne.CanvasObject) int {
	total := 0
	for _, obj := range objetos {
		if strings.HasSuffix(fmt.Sprintf("%T", obj), ".Shadow") {
			total++
		}
	}
	return total
}

// TestScrollSinSombras comprueba que el Scroll de lectura no dibuja los
// degradados de borde que Fyne añade por defecto (el inferior se veía como una
// franja despegada de la barra de navegación).
func TestScrollSinSombras(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	contenido := container.NewVBox(widget.NewLabel("página"))

	// El Scroll original de Fyne sí crea sombras: sirve de referencia.
	original := container.NewScroll(contenido)
	if n := contarSombras(original.CreateRenderer().Objects()); n == 0 {
		t.Fatal("el Scroll de Fyne debería tener sombras de borde")
	}

	s := ui.NewScrollSinSombras(contenido)
	objetos := s.CreateRenderer().Objects()
	if n := contarSombras(objetos); n != 0 {
		t.Fatalf("el Scroll de lectura no debe dibujar sombras, quedan %d", n)
	}
	if len(objetos) == 0 || objetos[0] != contenido {
		t.Fatal("el contenido debe seguir siendo el primer objeto del renderizador")
	}
}

// TestScrollSinSombrasDesplaza verifica que el widget derivado conserva el
// desplazamiento: el contenido se mueve y el Offset cambia.
func TestScrollSinSombrasDesplaza(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	contenido := container.NewVBox(widget.NewLabel("página"))
	for i := 0; i < 40; i++ {
		contenido.Add(widget.NewLabel("línea"))
	}

	s := ui.NewScrollSinSombras(contenido)
	s.Direction = container.ScrollVerticalOnly
	s.Resize(fyne.NewSize(200, 120))
	s.CreateRenderer()

	s.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.NewDelta(0, -30)})
	if s.Offset.Y <= 0 {
		t.Fatalf("el Scroll debe desplazarse, Offset.Y=%v", s.Offset.Y)
	}

	s.ScrollToOffset(fyne.NewPos(0, 0))
	if s.Offset.Y != 0 {
		t.Fatalf("ScrollToOffset debe volver al inicio, Offset.Y=%v", s.Offset.Y)
	}
}
