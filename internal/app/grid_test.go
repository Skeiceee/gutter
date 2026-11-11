package app

import (
	"errors"
	"image"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"gutter/internal/backend"
)

// openTestGrid deja la rejilla en el estado que tendría justo después de
// pulsar M, pero sin lanzar la carga asíncrona. Así las pruebas de navegación
// no dependen de un cómic real ni de temporizadores.
func openTestGrid(t *testing.T, pages int) (*gutterApp, *backend.Comic) {
	t.Helper()

	fyneApp := test.NewApp()
	t.Cleanup(fyneApp.Quit)

	g := NewGutterApp(fyneApp)
	names := make([]string, pages)
	for i := range names {
		names[i] = "page.png"
	}
	comic := &backend.Comic{Path: "test.cbz", Pages: names}
	g.comic = comic

	g.gridOpen = true
	g.gridComic = comic
	g.gridThumbSize = defaultGridThumbSize
	g.gridSelection = 0
	g.gridCells = make([]*thumbnailCell, pages)
	g.gridLoaded = make([]bool, pages)
	g.gridFailed = make([]bool, pages)
	g.gridLoading = make([]bool, pages)

	cellSize := gridCellSize(g.gridThumbSize)
	objects := make([]fyne.CanvasObject, pages)
	for i := range g.gridCells {
		g.gridCells[i] = newThumbnailCell(i, "page.png", cellSize, g.tapGridCell)
		objects[i] = g.gridCells[i]
	}
	g.gridStatus = widget.NewLabel("")

	// Un ancho fijo hace que gridColumns sea determinista.
	g.gridScroll = container.NewVScroll(container.NewGridWrap(cellSize, objects...))
	g.gridScroll.Resize(fyne.NewSize(800, 600))
	return g, comic
}

func TestThumbnailCellStates(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	cell := newThumbnailCell(4, "page5.png", fyne.NewSize(180, 260), nil)
	if got := cell.number.Text; got != "5" {
		t.Fatalf("número = %q, quiero %q", got, "5")
	}
	if cell.border.Visible() {
		t.Fatal("la celda no debe nacer seleccionada")
	}

	cell.SetSelected(true)
	if !cell.selected || !cell.border.Visible() {
		t.Fatal("SetSelected(true) debe mostrar el borde")
	}
	cell.SetSelected(false)
	if cell.selected || cell.border.Visible() {
		t.Fatal("SetSelected(false) debe ocultar el borde")
	}

	// Una imagen más ancha que alta se marca como página doble.
	spread := image.NewRGBA(image.Rect(0, 0, 120, 60))
	cell.SetThumbnail(spread)
	if !cell.loaded || cell.failed {
		t.Fatalf("estado tras miniatura = loaded %v, failed %v", cell.loaded, cell.failed)
	}
	if !cell.spreadPage || !cell.spread.Visible() {
		t.Fatal("una imagen horizontal debe marcarse como doble")
	}
	if !cell.image.Visible() || cell.errorLabel.Visible() {
		t.Fatal("la imagen debe verse y el error ocultarse")
	}

	vertical := image.NewRGBA(image.Rect(0, 0, 60, 120))
	cell.SetThumbnail(vertical)
	if cell.spreadPage || cell.spread.Visible() {
		t.Fatal("una imagen vertical no debe marcarse como doble")
	}

	cell.SetError(nil)
	if !cell.failed || cell.image.Visible() || !cell.errorLabel.Visible() {
		t.Fatal("SetError debe ocultar la imagen y mostrar el aviso")
	}
}

func TestGridCellSizeReservesLabelSpace(t *testing.T) {
	small := gridCellSize(120)
	large := gridCellSize(240)
	if large.Width <= small.Width || large.Height <= small.Height {
		t.Fatalf("el tamaño debe crecer con la miniatura: %v vs %v", small, large)
	}
	if small.Height <= small.Width {
		t.Fatalf("la celda debe dejar sitio a páginas verticales: %v", small)
	}
}

func TestGridNavigationByColumns(t *testing.T) {
	g, _ := openTestGrid(t, 100)

	columns := g.gridColumns()
	if columns < 2 {
		t.Fatalf("columnas = %d, se esperaban al menos 2 con 800 px", columns)
	}

	g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if g.gridSelection != 1 {
		t.Fatalf("derecha = %d, quiero 1", g.gridSelection)
	}
	g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if g.gridSelection != 1+columns {
		t.Fatalf("abajo = %d, quiero %d", g.gridSelection, 1+columns)
	}
	g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	if g.gridSelection != 1 {
		t.Fatalf("arriba = %d, quiero 1", g.gridSelection)
	}
	g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if g.gridSelection != 0 {
		t.Fatalf("izquierda = %d, quiero 0", g.gridSelection)
	}

	// Los extremos se recortan; nunca se sale del cómic.
	g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if g.gridSelection != 0 {
		t.Fatalf("izquierda en el límite = %d, quiero 0", g.gridSelection)
	}
	g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
	if g.gridSelection != 99 {
		t.Fatalf("Fin = %d, quiero 99", g.gridSelection)
	}
	g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if g.gridSelection != 99 {
		t.Fatalf("derecha en el límite = %d, quiero 99", g.gridSelection)
	}
	g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyHome})
	if g.gridSelection != 0 {
		t.Fatalf("Inicio = %d, quiero 0", g.gridSelection)
	}
}

func TestGridKeyHandlingClosesAndConsumes(t *testing.T) {
	g, _ := openTestGrid(t, 6)

	if !g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyRight}) {
		t.Fatal("con la rejilla abierta, cualquier tecla debe consumirse")
	}
	g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if g.gridOpen {
		t.Fatal("Esc debe cerrar la rejilla")
	}
	if g.gridCells != nil || g.gridScroll != nil || g.gridComic != nil {
		t.Fatal("al cerrar se debe liberar el estado de la rejilla")
	}
	if g.handleGridKey(&fyne.KeyEvent{Name: fyne.KeyRight}) {
		t.Fatal("con la rejilla cerrada, las teclas no deben consumirse")
	}
}

func TestGridApplyResults(t *testing.T) {
	g, comic := openTestGrid(t, 3)
	g.gridPending = 2
	seq := g.gridSeq.Add(1)

	thumb := image.NewRGBA(image.Rect(0, 0, 80, 40))
	g.applyGridThumbnail(seq, comic, 0, thumb, nil)
	if !g.gridLoaded[0] || g.gridFailed[0] {
		t.Fatalf("celda 0 = loaded %v, failed %v", g.gridLoaded[0], g.gridFailed[0])
	}
	if g.gridPending != 1 {
		t.Fatalf("pendientes = %d, quiero 1", g.gridPending)
	}

	g.applyGridThumbnail(seq, comic, 1, nil, errFakeThumb)
	if !g.gridLoaded[1] || !g.gridFailed[1] {
		t.Fatalf("celda 1 = loaded %v, failed %v", g.gridLoaded[1], g.gridFailed[1])
	}
	if g.gridPending != 0 {
		t.Fatalf("pendientes = %d, quiero 0", g.gridPending)
	}

	status := g.gridStatus.Text
	if !strings.Contains(status, "Página 1 de 3") {
		t.Fatalf("estado = %q, falta el número de página", status)
	}
	if !strings.Contains(status, "1 con error") {
		t.Fatalf("estado = %q, falta el recuento de errores", status)
	}

	// Un resultado de una sesión anterior no debe tocar la rejilla nueva.
	g.applyGridThumbnail(seq-1, comic, 2, thumb, nil)
	if g.gridLoaded[2] {
		t.Fatal("un lote viejo no debe aplicar resultados")
	}
}

func TestGridVisibleRangeIncludesMargin(t *testing.T) {
	g, _ := openTestGrid(t, 100)
	columns := g.gridColumns()
	rowHeight := g.gridRowHeight()

	start, end := g.gridVisibleRange()
	if start != 0 {
		t.Fatalf("inicio sin desplazar = %d, quiero 0", start)
	}
	if end < columns {
		t.Fatalf("fin = %d, debe cubrir al menos una fila (%d)", end, columns)
	}

	// Tres filas abajo: el margen superior de dos filas debe incluirse.
	g.gridScroll.Offset = fyne.NewPos(0, rowHeight*3)
	start, end = g.gridVisibleRange()
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

	g := NewGutterApp(fyneApp)
	g.comic = &backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png", "3.png"}}
	g.pageIndex = 1

	g.showThumbnailGrid()
	if !g.gridOpen {
		t.Fatal("showThumbnailGrid debe abrir la rejilla")
	}
	if len(g.gridCells) != 3 {
		t.Fatalf("celdas = %d, quiero 3", len(g.gridCells))
	}
	if g.gridSelection != 1 {
		t.Fatalf("selección inicial = %d, quiero la página actual (1)", g.gridSelection)
	}
	if !g.gridOverlay.Visible() {
		t.Fatal("la capa de la rejilla debe hacerse visible")
	}

	g.hideThumbnailGrid()
	if g.gridOpen || g.gridOverlay.Visible() {
		t.Fatal("hideThumbnailGrid debe cerrar y ocultar la capa")
	}
	if g.gridOverlay.Objects != nil {
		t.Fatal("la capa debe liberar sus objetos al cerrarse")
	}

	// El centrado diferido no debe reabrir ni repoblar una rejilla cerrada.
	time.Sleep(90 * time.Millisecond)
	if g.gridOpen || g.gridCells != nil {
		t.Fatal("el temporizador no debe actuar sobre una rejilla cerrada")
	}

	// Sin cómic no hay nada que mostrar.
	g.comic = nil
	g.showThumbnailGrid()
	if g.gridOpen {
		t.Fatal("sin cómic la rejilla no debe abrirse")
	}
}

func TestGridThumbSizePreference(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := NewGutterApp(fyneApp)
	if got := g.gridThumbSizePref(); got != defaultGridThumbSize {
		t.Fatalf("tamaño por defecto = %d, quiero %d", got, defaultGridThumbSize)
	}

	g.prefs.SetInt(prefGridThumbSize, 240)
	if got := g.gridThumbSizePref(); got != 240 {
		t.Fatalf("tamaño guardado = %d, quiero 240", got)
	}

	// Un valor desconocido (por ejemplo, de una versión futura) vuelve al
	// tamaño seguro en vez de romper la geometría.
	g.prefs.SetInt(prefGridThumbSize, 999)
	if got := g.gridThumbSizePref(); got != defaultGridThumbSize {
		t.Fatalf("tamaño inválido = %d, quiero %d", got, defaultGridThumbSize)
	}

	if got := gridThumbLabel(200); got != "Grandes (200 px)" {
		t.Fatalf("etiqueta = %q", got)
	}
	if got := parseGridThumbSize("Muy grandes (240 px)"); got != 240 {
		t.Fatalf("tamaño analizado = %d, quiero 240", got)
	}
	if got := parseGridThumbSize("desconocido"); got != defaultGridThumbSize {
		t.Fatalf("tamaño desconocido = %d, quiero %d", got, defaultGridThumbSize)
	}
}

// errFakeThumb simula el fallo de decodificación de una miniatura.
var errFakeThumb = errors.New("miniatura de prueba rota")
