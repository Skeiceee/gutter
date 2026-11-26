package tests

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/ui"
)

// En modo limpio la barra se oculta y el área de lectura debe ocupar toda la
// altura. Antes quedaba una franja de 40 px con el fondo de la ventana.
func TestRootLayoutReaderFillsHeightWhenBarHidden(t *testing.T) {
	reader := canvas.NewRectangle(color.Transparent)
	bar := container.NewVBox(widget.NewLabel("barra"))
	bar.Hide()

	layout := &ui.RootLayout{}
	size := fyne.NewSize(800, 600)
	layout.Layout([]fyne.CanvasObject{reader, bar}, size)

	if got := reader.Size().Height; got != size.Height {
		t.Fatalf("el área de lectura debería medir %v de alto sin barra, mide %v", size.Height, got)
	}
	if bar.Visible() {
		t.Fatal("la barra debería seguir oculta")
	}
}

func TestRootLayoutReservesBarHeightWhenVisible(t *testing.T) {
	reader := canvas.NewRectangle(color.Transparent)
	bar := container.NewVBox(widget.NewLabel("barra"))

	layout := &ui.RootLayout{}
	size := fyne.NewSize(800, 600)
	layout.Layout([]fyne.CanvasObject{reader, bar}, size)

	barH := bar.MinSize().Height
	if barH < 1 {
		barH = 40
	}
	want := size.Height - barH
	if got := reader.Size().Height; got != want {
		t.Fatalf("el área de lectura debería medir %v con la barra visible, mide %v", want, got)
	}
	if got := bar.Size().Height; got != barH {
		t.Fatalf("la barra debería medir %v, mide %v", barH, got)
	}
}
