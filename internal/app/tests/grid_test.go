package tests

import (
	"errors"
	"image"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"gutter/internal/app"
	"gutter/internal/backend"
)

// openTestGrid deja la rejilla en el estado que tendría justo después de
// pulsar M, pero sin lanzar la carga asíncrona.
func openTestGrid(t *testing.T, pages int) (*app.TestApp, *backend.Comic) {
	t.Helper()

	fyneApp := test.NewApp()
	t.Cleanup(fyneApp.Quit)

	g := app.NewTestApp(fyneApp)
	return g, g.InitGrid(pages)
}

func TestThumbnailCellStates(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	cell := app.NewTestCell(4, "page5.png", fyne.NewSize(180, 260))
	if got := cell.NumberText(); got != "5" {
		t.Fatalf("número = %q, quiero %q", got, "5")
	}
	if cell.BorderVisible() {
		t.Fatal("la celda no debe nacer seleccionada")
	}

	cell.SetSelected(true)
	if !cell.Selected() || !cell.BorderVisible() {
		t.Fatal("SetSelected(true) debe mostrar el borde")
	}
	cell.SetSelected(false)
	if cell.Selected() || cell.BorderVisible() {
		t.Fatal("SetSelected(false) debe ocultar el borde")
	}

	// Una imagen más ancha que alta se marca como página doble.
	spread := image.NewRGBA(image.Rect(0, 0, 120, 60))
	cell.SetThumbnail(spread)
	if !cell.Loaded() || cell.Failed() {
		t.Fatalf("estado tras miniatura = loaded %v, failed %v", cell.Loaded(), cell.Failed())
	}
	if !cell.SpreadPage() || !cell.SpreadVisible() {
		t.Fatal("una imagen horizontal debe marcarse como doble")
	}
	if !cell.ImageVisible() || cell.ErrorVisible() {
		t.Fatal("la imagen debe verse y el error ocultarse")
	}

	vertical := image.NewRGBA(image.Rect(0, 0, 60, 120))
	cell.SetThumbnail(vertical)
	if cell.SpreadPage() || cell.SpreadVisible() {
		t.Fatal("una imagen vertical no debe marcarse como doble")
	}

	cell.SetError(nil)
	if !cell.Failed() || cell.ImageVisible() || !cell.ErrorVisible() {
		t.Fatal("SetError debe ocultar la imagen y mostrar el aviso")
	}
}

func TestGridCellSizeReservesLabelSpace(t *testing.T) {
	small := app.GridCellSize(120)
	large := app.GridCellSize(240)
	if large.Width <= small.Width || large.Height <= small.Height {
		t.Fatalf("el tamaño debe crecer con la miniatura: %v vs %v", small, large)
	}
	if small.Height <= small.Width {
		t.Fatalf("la celda debe dejar sitio a páginas verticales: %v", small)
	}
}

// La resolución pedida al backend debe cubrir el alto real del marco y el
// factor de escala de la pantalla; si no, Fyne amplía la miniatura y se ve
// borrosa.
func TestGridThumbnailRenderSizeCoversCell(t *testing.T) {
	g, _ := openTestGrid(t, 4)

	for _, size := range app.GridThumbValues {
		g.SetGridThumbSize(size)
		render := g.GridThumbnailRenderSize()
		imageHeight := app.GridCellSize(size).Height - 36 // espacio del número y márgenes
		if float32(render) < imageHeight {
			t.Errorf("tamaño %d: render %d < alto de imagen %.0f", size, render, imageHeight)
		}
		if render > 1024 {
			t.Errorf("tamaño %d: render %d supera el máximo del backend", size, render)
		}
	}
}

func TestGridNavigationByColumns(t *testing.T) {
	g, _ := openTestGrid(t, 100)

	columns := g.GridColumns()
	if columns < 2 {
		t.Fatalf("columnas = %d, se esperaban al menos 2 con 800 px", columns)
	}

	g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if g.GridSelection() != 1 {
		t.Fatalf("derecha = %d, quiero 1", g.GridSelection())
	}
	g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if g.GridSelection() != 1+columns {
		t.Fatalf("abajo = %d, quiero %d", g.GridSelection(), 1+columns)
	}
	g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	if g.GridSelection() != 1 {
		t.Fatalf("arriba = %d, quiero 1", g.GridSelection())
	}
	g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if g.GridSelection() != 0 {
		t.Fatalf("izquierda = %d, quiero 0", g.GridSelection())
	}

	// Los extremos se recortan; nunca se sale del cómic.
	g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if g.GridSelection() != 0 {
		t.Fatalf("izquierda en el límite = %d, quiero 0", g.GridSelection())
	}
	g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
	if g.GridSelection() != 99 {
		t.Fatalf("Fin = %d, quiero 99", g.GridSelection())
	}
	g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if g.GridSelection() != 99 {
		t.Fatalf("derecha en el límite = %d, quiero 99", g.GridSelection())
	}
	g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyHome})
	if g.GridSelection() != 0 {
		t.Fatalf("Inicio = %d, quiero 0", g.GridSelection())
	}
}

func TestGridKeyHandlingClosesAndConsumes(t *testing.T) {
	g, _ := openTestGrid(t, 6)

	if !g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyRight}) {
		t.Fatal("con la rejilla abierta, cualquier tecla debe consumirse")
	}
	g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if g.GridOpen() {
		t.Fatal("Esc debe cerrar la rejilla")
	}
	if !g.GridStateCleared() {
		t.Fatal("al cerrar se debe liberar el estado de la rejilla")
	}
	if g.HandleGridKey(&fyne.KeyEvent{Name: fyne.KeyRight}) {
		t.Fatal("con la rejilla cerrada, las teclas no deben consumirse")
	}
}

func TestGridApplyResults(t *testing.T) {
	g, comic := openTestGrid(t, 3)
	g.SetGridPending(2)
	seq := g.GridSeqAdd(1)

	thumb := image.NewRGBA(image.Rect(0, 0, 80, 40))
	g.ApplyGridThumbnail(seq, comic, 0, thumb, nil)
	if !g.GridLoaded(0) || g.GridFailed(0) {
		t.Fatalf("celda 0 = loaded %v, failed %v", g.GridLoaded(0), g.GridFailed(0))
	}
	if g.GridPending() != 1 {
		t.Fatalf("pendientes = %d, quiero 1", g.GridPending())
	}

	g.ApplyGridThumbnail(seq, comic, 1, nil, errFakeThumb)
	if !g.GridLoaded(1) || !g.GridFailed(1) {
		t.Fatalf("celda 1 = loaded %v, failed %v", g.GridLoaded(1), g.GridFailed(1))
	}
	if g.GridPending() != 0 {
		t.Fatalf("pendientes = %d, quiero 0", g.GridPending())
	}

	status := g.GridStatusText()
	if !strings.Contains(status, "Página 1 de 3") {
		t.Fatalf("estado = %q, falta el número de página", status)
	}
	if !strings.Contains(status, "1 con error") {
		t.Fatalf("estado = %q, falta el recuento de errores", status)
	}

	// Un resultado de una sesión anterior no debe tocar la rejilla nueva.
	g.ApplyGridThumbnail(seq-1, comic, 2, thumb, nil)
	if g.GridLoaded(2) {
		t.Fatal("un lote viejo no debe aplicar resultados")
	}
}

func TestGridVisibleRangeIncludesMargin(t *testing.T) {
	g, _ := openTestGrid(t, 100)
	columns := g.GridColumns()
	rowHeight := g.GridRowHeight()

	start, end := g.GridVisibleRange()
	if start != 0 {
		t.Fatalf("inicio sin desplazar = %d, quiero 0", start)
	}
	if end < columns {
		t.Fatalf("fin = %d, debe cubrir al menos una fila (%d)", end, columns)
	}

	// Tres filas abajo: el margen superior de dos filas debe incluirse.
	g.SetGridScrollOffsetY(rowHeight * 3)
	start, end = g.GridVisibleRange()
	wantStart := columns // (fila 3 - margen 2) * columnas
	if start != wantStart {
		t.Fatalf("inicio con desplazamiento = %d, quiero %d", start, wantStart)
	}
	if end <= start || end >= 100 {
		t.Fatalf("fin con desplazamiento = %d, fuera de rango", end)
	}
}

func TestGridLifecycle(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	g.SetComic(&backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png", "3.png"}})
	g.SetPageIndex(1)

	g.ShowThumbnailGrid()
	if !g.GridOpen() {
		t.Fatal("ShowThumbnailGrid debe abrir la rejilla")
	}
	if g.GridCellCount() != 3 {
		t.Fatalf("celdas = %d, quiero 3", g.GridCellCount())
	}
	if g.GridSelection() != 1 {
		t.Fatalf("selección inicial = %d, quiero la página actual (1)", g.GridSelection())
	}
	if !g.GridOverlayVisible() {
		t.Fatal("la capa de la rejilla debe hacerse visible")
	}

	g.HideThumbnailGrid()
	if g.GridOpen() || g.GridOverlayVisible() {
		t.Fatal("HideThumbnailGrid debe cerrar y ocultar la capa")
	}
	if !g.GridOverlayObjectsNil() {
		t.Fatal("la capa debe liberar sus objetos al cerrarse")
	}

	// El centrado diferido no debe reabrir ni repoblar una rejilla cerrada.
	time.Sleep(90 * time.Millisecond)
	if g.GridOpen() || !g.GridCellsNil() {
		t.Fatal("el temporizador no debe actuar sobre una rejilla cerrada")
	}

	// Sin cómic no hay nada que mostrar.
	g.SetComic(nil)
	g.ShowThumbnailGrid()
	if g.GridOpen() {
		t.Fatal("sin cómic la rejilla no debe abrirse")
	}
}

func TestGridThumbSizePreference(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	if got := g.GridThumbSizePref(); got != app.DefaultGridThumbSize {
		t.Fatalf("tamaño por defecto = %d, quiero %d", got, app.DefaultGridThumbSize)
	}

	g.SetPrefInt(app.PrefGridThumbSize, 240)
	if got := g.GridThumbSizePref(); got != 240 {
		t.Fatalf("tamaño guardado = %d, quiero 240", got)
	}

	// Un valor desconocido (por ejemplo, de una versión futura) vuelve al
	// tamaño seguro en vez de romper la geometría.
	g.SetPrefInt(app.PrefGridThumbSize, 999)
	if got := g.GridThumbSizePref(); got != app.DefaultGridThumbSize {
		t.Fatalf("tamaño inválido = %d, quiero %d", got, app.DefaultGridThumbSize)
	}

	if got := app.GridThumbLabel(200); got != "Grandes (200 px)" {
		t.Fatalf("etiqueta = %q", got)
	}
	if got := app.ParseGridThumbSize("Muy grandes (240 px)"); got != 240 {
		t.Fatalf("tamaño analizado = %d, quiero 240", got)
	}
	if got := app.ParseGridThumbSize("desconocido"); got != app.DefaultGridThumbSize {
		t.Fatalf("tamaño desconocido = %d, quiero %d", got, app.DefaultGridThumbSize)
	}
}

// errFakeThumb simula el fallo de decodificación de una miniatura.
var errFakeThumb = errors.New("miniatura de prueba rota")
