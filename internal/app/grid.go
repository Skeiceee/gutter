// grid.go implementa la rejilla de páginas: una vista general del cómic con
// miniaturas, carga perezosa alrededor de la zona visible y navegación por
// teclado. El backend ya genera las miniaturas con una caché separada; esta
// capa solo se ocupa de pedirlas en el orden y el momento adecuados.
package app

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/backend"
)

const (
	gridLoadMarginRows = 2
	gridBottomLabel    = "↑↓←→ mover · Enter abrir · M/Esc cerrar"
)

// thumbnailCell es una celda dibujada a mano. No usa widget.Button para no
// añadir el fondo y el padding que Fyne aplica a los botones: la imagen debe
// ocupar casi toda la celda y el borde azul solo marca la selección.
type thumbnailCell struct {
	widget.BaseWidget

	index    int
	name     string
	cellSize fyne.Size
	onTap    func(int)

	bg         *canvas.Rectangle
	border     *canvas.Rectangle
	image      *canvas.Image
	number     *canvas.Text
	numberBG   *canvas.Rectangle
	spread     *canvas.Text
	spreadBG   *canvas.Rectangle
	errorLabel *canvas.Text
	selected   bool
	loaded     bool
	failed     bool
	spreadPage bool
}

func newThumbnailCell(index int, name string, cellSize fyne.Size, onTap func(int)) *thumbnailCell {
	cell := &thumbnailCell{
		index:    index,
		name:     name,
		cellSize: cellSize,
		onTap:    onTap,
	}
	cell.ExtendBaseWidget(cell)

	cell.bg = canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	cell.bg.CornerRadius = 6

	cell.border = canvas.NewRectangle(color.Transparent)
	cell.border.CornerRadius = 6
	cell.border.StrokeColor = theme.Color(theme.ColorNamePrimary)
	cell.border.StrokeWidth = 2
	cell.border.Hide()

	cell.image = canvas.NewImageFromImage(nil)
	cell.image.FillMode = canvas.ImageFillContain
	// Las miniaturas se generan con resolución de sobra y Fyne las reduce; el
	// filtro suave evita el aspecto escalonado al cambiar de tamaño.
	cell.image.ScaleMode = canvas.ImageScaleSmooth
	cell.image.Hide()

	cell.number = canvas.NewText(fmt.Sprintf("%d", index+1), theme.Color(theme.ColorNameForeground))
	cell.number.TextSize = 13
	cell.number.TextStyle = fyne.TextStyle{Bold: true}
	cell.number.Alignment = fyne.TextAlignCenter
	cell.numberBG = canvas.NewRectangle(color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xB8})
	cell.numberBG.CornerRadius = 4

	cell.spread = canvas.NewText("DOBLE", theme.Color(theme.ColorNameForegroundOnPrimary))
	cell.spread.TextSize = 10
	cell.spread.TextStyle = fyne.TextStyle{Bold: true}
	cell.spread.Alignment = fyne.TextAlignCenter
	cell.spreadBG = canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	cell.spreadBG.CornerRadius = 4
	cell.spread.Hide()
	cell.spreadBG.Hide()

	cell.errorLabel = canvas.NewText("No se pudo cargar", theme.Color(theme.ColorNameError))
	cell.errorLabel.TextSize = 12
	cell.errorLabel.TextStyle = fyne.TextStyle{Bold: true}
	cell.errorLabel.Alignment = fyne.TextAlignCenter
	cell.errorLabel.Hide()

	return cell
}

func (c *thumbnailCell) CreateRenderer() fyne.WidgetRenderer {
	return &thumbnailCellRenderer{cell: c}
}

func (c *thumbnailCell) Tapped(_ *fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap(c.index)
	}
}

func (c *thumbnailCell) SetThumbnail(img *image.RGBA) {
	if img == nil {
		c.SetError(fmt.Errorf("miniatura vacía"))
		return
	}
	c.loaded = true
	c.failed = false
	c.spreadPage = img.Bounds().Dx() > img.Bounds().Dy()
	c.image.Image = img
	c.image.Show()
	c.errorLabel.Hide()
	if c.spreadPage {
		c.spread.Show()
		c.spreadBG.Show()
	} else {
		c.spread.Hide()
		c.spreadBG.Hide()
	}
	c.Refresh()
}

func (c *thumbnailCell) SetError(err error) {
	c.loaded = true
	c.failed = true
	_ = err // el detalle se ve al abrir la página, que ofrece Reintentar
	c.image.Hide()
	c.errorLabel.Show()
	c.spread.Hide()
	c.spreadBG.Hide()
	c.Refresh()
}

func (c *thumbnailCell) SetSelected(selected bool) {
	if c.selected == selected {
		return
	}
	c.selected = selected
	if selected {
		c.border.Show()
	} else {
		c.border.Hide()
	}
	c.Refresh()
}

func (c *thumbnailCell) MinSize() fyne.Size {
	return c.cellSize
}

type thumbnailCellRenderer struct {
	cell    *thumbnailCell
	objects []fyne.CanvasObject
}

func (r *thumbnailCellRenderer) Layout(size fyne.Size) {
	c := r.cell
	c.bg.Resize(size)
	c.bg.Move(fyne.NewPos(0, 0))

	labelSpace := float32(30)
	imageSize := fyne.NewSize(
		fyne.Max(1, size.Width-12),
		fyne.Max(1, size.Height-labelSpace-6),
	)
	c.image.Resize(imageSize)
	c.image.Move(fyne.NewPos(6, 6))

	numberSize := c.number.MinSize()
	numberBGSize := fyne.NewSize(numberSize.Width+10, numberSize.Height+4)
	c.numberBG.Resize(numberBGSize)
	c.numberBG.Move(fyne.NewPos((size.Width-numberBGSize.Width)/2, size.Height-numberBGSize.Height-5))
	c.number.Resize(numberSize)
	c.number.Move(fyne.NewPos((size.Width-numberSize.Width)/2, size.Height-numberSize.Height-5))

	spreadSize := c.spread.MinSize()
	spreadBGSize := fyne.NewSize(spreadSize.Width+10, spreadSize.Height+4)
	c.spreadBG.Resize(spreadBGSize)
	c.spreadBG.Move(fyne.NewPos(size.Width-spreadBGSize.Width-6, 6))
	c.spread.Resize(spreadSize)
	c.spread.Move(fyne.NewPos(size.Width-spreadSize.Width-11, 8))

	c.errorLabel.Resize(fyne.NewSize(fyne.Max(1, size.Width-16), 20))
	c.errorLabel.Move(fyne.NewPos(8, (size.Height-labelSpace-20)/2))

	c.border.Resize(size)
	c.border.Move(fyne.NewPos(0, 0))
}

func (r *thumbnailCellRenderer) MinSize() fyne.Size {
	return r.cell.cellSize
}

func (r *thumbnailCellRenderer) Refresh() {
	r.Layout(r.cell.Size())
}

func (r *thumbnailCellRenderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		r.objects = []fyne.CanvasObject{
			r.cell.bg,
			r.cell.image,
			r.cell.numberBG,
			r.cell.number,
			r.cell.spreadBG,
			r.cell.spread,
			r.cell.errorLabel,
			r.cell.border,
		}
	}
	return r.objects
}

func (r *thumbnailCellRenderer) Destroy() {}

// gridCellSize reserva altura para páginas verticales y para el número. Las
// páginas dobles se ajustan dentro del mismo marco con ImageFillContain.
func gridCellSize(thumbSize int) fyne.Size {
	return fyne.NewSize(float32(thumbSize)+28, float32(thumbSize)*1.45+34)
}

// gridThumbnailRenderSize calcula la resolución real que se pide al backend.
// El marco de la celda deja que una página vertical crezca hasta ~1,45 veces
// el tamaño lógico y la pantalla puede estar al 150 % o 200 %, así que generar
// solo "thumbSize" píxeles obligaba a Fyne a ampliar la imagen y se veía
// borrosa. El factor 1,5 cubre ambos casos y deja margen para el filtro suave.
func (g *gutterApp) gridThumbnailRenderSize() int {
	scale := float64(1)
	if g.window != nil && g.window.Canvas() != nil {
		if s := g.window.Canvas().Scale(); s > 0 {
			scale = float64(s)
		}
	}
	render := int(math.Ceil(float64(g.gridThumbSize) * scale * 1.5))
	return clampInt(render, g.gridThumbSize, 1024)
}

func (g *gutterApp) toggleThumbnailGrid() {
	if g.gridOpen {
		g.hideThumbnailGrid()
		return
	}
	g.showThumbnailGrid()
}

// showThumbnailGrid construye la capa y lanza la primera carga. La geometría
// real del scroll no está lista hasta el siguiente frame, así que una pequeña
// espera programa el centrado y la carga de la zona visible.
func (g *gutterApp) showThumbnailGrid() {
	if g.comic == nil || len(g.comic.Pages) == 0 || g.gridOpen || g.loading {
		return
	}
	g.hideActiveDialog()
	g.closeGotoPage()

	g.gridOpen = true
	g.gridComic = g.comic
	g.gridThumbSize = g.gridThumbSizePref()
	g.gridThumbRender = g.gridThumbnailRenderSize()
	g.gridSelection = clampInt(g.pageIndex, 0, len(g.comic.Pages)-1)

	ctx, cancel := context.WithCancel(context.Background())
	g.gridCtx = ctx
	g.gridCancel = cancel
	seq := g.gridSeq.Add(1)

	cellSize := gridCellSize(g.gridThumbSize)
	g.gridCells = make([]*thumbnailCell, len(g.comic.Pages))
	g.gridLoaded = make([]bool, len(g.comic.Pages))
	g.gridFailed = make([]bool, len(g.comic.Pages))
	g.gridLoading = make([]bool, len(g.comic.Pages))
	objects := make([]fyne.CanvasObject, len(g.comic.Pages))
	for i, name := range g.comic.Pages {
		cell := newThumbnailCell(i, name, cellSize, g.tapGridCell)
		cell.SetSelected(i == g.gridSelection)
		g.gridCells[i] = cell
		objects[i] = cell
	}

	title := widget.NewLabelWithStyle("Rejilla de páginas", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	count := widget.NewLabel(fmt.Sprintf("%d páginas", len(g.comic.Pages)))
	count.Importance = widget.LowImportance
	closeButton := widget.NewButtonWithIcon("Cerrar", theme.CancelIcon(), g.hideThumbnailGrid)
	closeButton.Importance = widget.LowImportance

	header := container.NewBorder(
		nil, nil,
		container.NewHBox(title, count),
		closeButton,
	)

	g.gridStatus = widget.NewLabel("")
	g.gridStatus.Importance = widget.LowImportance
	hint := widget.NewLabel(gridBottomLabel)
	hint.Importance = widget.LowImportance
	hint.Alignment = fyne.TextAlignTrailing
	footer := container.NewBorder(nil, nil, g.gridStatus, hint)

	g.gridScroll = container.NewVScroll(container.NewGridWrap(cellSize, objects...))
	g.gridScroll.Direction = container.ScrollVerticalOnly
	g.gridScroll.OnScrolled = func(fyne.Position) {
		if g.gridOpen {
			g.loadGridVisible()
		}
	}

	body := container.NewBorder(header, footer, nil, nil, g.gridScroll)
	background := canvas.NewRectangle(color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xF4})
	g.gridOverlay.Objects = []fyne.CanvasObject{background, container.NewPadded(body)}
	g.gridOverlay.Show()
	g.root.Refresh()
	g.updateGridStatus()

	time.AfterFunc(60*time.Millisecond, func() {
		fyne.Do(func() {
			if !g.gridOpen || seq != g.gridSeq.Load() {
				return
			}
			g.scrollGridToSelection()
			// En un RAR sólido cada lote vuelve a recorrer el contenedor desde
			// el principio. Es mejor generar todo en una sola pasada y dejar
			// que las miniaturas aparezcan en orden.
			if g.gridComic != nil && g.gridComic.IsSolid() {
				g.loadGridRange(0, len(g.gridCells)-1)
			} else {
				g.loadGridVisible()
			}
		})
	})
}

func (g *gutterApp) hideThumbnailGrid() {
	if !g.gridOpen {
		return
	}
	g.gridOpen = false
	g.gridSeq.Add(1)
	if g.gridCancel != nil {
		g.gridCancel()
		g.gridCancel = nil
	}
	g.gridCtx = nil
	if g.gridOverlay != nil {
		g.gridOverlay.Hide()
		g.gridOverlay.Objects = nil
		g.gridOverlay.Refresh()
	}
	if g.root != nil {
		g.root.Refresh()
	}
	g.gridScroll = nil
	g.gridCells = nil
	g.gridLoaded = nil
	g.gridFailed = nil
	g.gridLoading = nil
	g.gridSelection = 0
	g.gridThumbRender = 0
	g.gridPending = 0
	g.gridStatus = nil
	g.gridComic = nil
	g.unfocus()
}

func (g *gutterApp) tapGridCell(index int) {
	if !g.gridOpen {
		return
	}
	g.setGridSelection(index, false)
	g.jumpFromGrid()
}

func (g *gutterApp) jumpFromGrid() {
	if !g.gridOpen || g.comic == nil {
		return
	}
	index := g.gridSelection
	g.hideThumbnailGrid()
	g.goToPage(index)
}

func (g *gutterApp) setGridSelection(index int, ensureVisible bool) {
	if !g.gridOpen || len(g.gridCells) == 0 {
		return
	}
	index = clampInt(index, 0, len(g.gridCells)-1)
	old := g.gridSelection
	g.gridSelection = index
	if old >= 0 && old < len(g.gridCells) {
		g.gridCells[old].SetSelected(false)
	}
	g.gridCells[index].SetSelected(true)
	if ensureVisible {
		g.scrollGridToSelection()
	}
	g.updateGridStatus()
	if ensureVisible {
		g.loadGridVisible()
	}
}

func (g *gutterApp) moveGridSelection(delta int) {
	if !g.gridOpen || len(g.gridCells) == 0 {
		return
	}
	g.setGridSelection(g.gridSelection+delta, true)
}

func (g *gutterApp) gridColumns() int {
	if g.gridScroll == nil {
		return 1
	}
	width := g.gridScroll.Size().Width
	if width <= 0 {
		width = g.window.Canvas().Size().Width - 32
	}
	cell := gridCellSize(g.gridThumbSize)
	pad := theme.Padding()
	columns := int((width + pad) / (cell.Width + pad))
	return clampInt(columns, 1, len(g.gridCells))
}

func (g *gutterApp) gridRowHeight() float32 {
	return gridCellSize(g.gridThumbSize).Height + theme.Padding()
}

func (g *gutterApp) scrollGridToSelection() {
	if g.gridScroll == nil || len(g.gridCells) == 0 {
		return
	}
	row := g.gridSelection / g.gridColumns()
	rowHeight := g.gridRowHeight()
	top := float32(row) * rowHeight
	bottom := top + gridCellSize(g.gridThumbSize).Height
	viewport := g.gridScroll.Size().Height
	if viewport <= 0 {
		viewport = 600
	}
	offset := g.gridScroll.Offset.Y
	switch {
	case top < offset:
		g.gridScroll.ScrollToOffset(fyne.NewPos(0, top))
	case bottom > offset+viewport:
		g.gridScroll.ScrollToOffset(fyne.NewPos(0, bottom-viewport+theme.Padding()))
	}
}

func (g *gutterApp) gridVisibleRange() (int, int) {
	if !g.gridOpen || len(g.gridCells) == 0 || g.gridScroll == nil {
		return 0, -1
	}
	columns := g.gridColumns()
	rowHeight := g.gridRowHeight()
	viewport := g.gridScroll.Size().Height
	if viewport <= 0 {
		viewport = 600
	}
	offset := g.gridScroll.Offset.Y
	if offset < 0 {
		offset = 0
	}
	firstRow := int(offset / rowHeight)
	lastRow := int((offset + viewport) / rowHeight)
	start := (firstRow - gridLoadMarginRows) * columns
	end := (lastRow+gridLoadMarginRows+1)*columns - 1
	if start < 0 {
		start = 0
	}
	if end >= len(g.gridCells) {
		end = len(g.gridCells) - 1
	}
	return start, end
}

func (g *gutterApp) loadGridVisible() {
	start, end := g.gridVisibleRange()
	g.loadGridRange(start, end)
}

// loadGridRange pide un bloque de miniaturas. Si el usuario se desplaza rápido
// pueden solaparse bloques; gridLoading evita pedir dos veces la misma celda.
func (g *gutterApp) loadGridRange(start, end int) {
	if !g.gridOpen || g.gridComic == nil || g.gridCtx == nil || end < start {
		return
	}
	start = clampInt(start, 0, len(g.gridCells)-1)
	end = clampInt(end, start, len(g.gridCells)-1)

	names := make([]string, 0, end-start+1)
	requested := make(map[string]int, end-start+1)
	for i := start; i <= end; i++ {
		if g.gridLoaded[i] || g.gridLoading[i] {
			continue
		}
		name := g.gridComic.Pages[i]
		g.gridLoading[i] = true
		names = append(names, name)
		requested[name] = i
	}
	if len(names) == 0 {
		return
	}

	g.gridPending += len(names)
	g.updateGridStatus()

	ctx := g.gridCtx
	comic := g.gridComic
	seq := g.gridSeq.Load()
	size := g.gridThumbRender
	if size <= 0 {
		size = g.gridThumbSize
	}

	go func() {
		done := make(map[int]bool, len(names))
		_ = comic.ThumbnailsContext(ctx, names, size, func(name string, img *image.RGBA, err error) {
			index, ok := requested[name]
			if !ok || done[index] {
				return
			}
			done[index] = true
			fyne.Do(func() {
				g.applyGridThumbnail(seq, comic, index, img, err)
			})
		})

		fyne.Do(func() {
			g.finishGridBatch(seq, comic, requested, done)
		})
	}()
}

func (g *gutterApp) applyGridThumbnail(seq uint64, comic *backend.Comic, index int, img *image.RGBA, err error) {
	if !g.gridIsCurrent(seq, comic) || index < 0 || index >= len(g.gridCells) {
		return
	}
	g.gridLoading[index] = false
	if g.gridPending > 0 {
		g.gridPending--
	}
	if err != nil {
		g.gridLoaded[index] = true
		g.gridFailed[index] = true
		g.gridCells[index].SetError(err)
	} else {
		g.gridLoaded[index] = true
		g.gridFailed[index] = false
		g.gridCells[index].SetThumbnail(img)
	}
	g.updateGridStatus()
}

func (g *gutterApp) finishGridBatch(seq uint64, comic *backend.Comic, requested map[string]int, done map[int]bool) {
	if !g.gridIsCurrent(seq, comic) {
		return
	}
	err := fmt.Errorf("no se pudo generar la miniatura")
	for _, index := range requested {
		if done[index] {
			continue
		}
		if index < 0 || index >= len(g.gridCells) {
			continue
		}
		g.gridLoading[index] = false
		if g.gridPending > 0 {
			g.gridPending--
		}
		g.gridLoaded[index] = true
		g.gridFailed[index] = true
		g.gridCells[index].SetError(err)
	}
	g.updateGridStatus()
}

func (g *gutterApp) gridIsCurrent(seq uint64, comic *backend.Comic) bool {
	return g.gridOpen && seq == g.gridSeq.Load() && g.gridComic == comic
}

func (g *gutterApp) updateGridStatus() {
	if g.gridStatus == nil || !g.gridOpen {
		return
	}
	loaded := 0
	failed := 0
	for i := range g.gridCells {
		if g.gridLoaded[i] {
			loaded++
		}
		if g.gridFailed[i] {
			failed++
		}
	}
	parts := []string{fmt.Sprintf("Página %d de %d", g.gridSelection+1, len(g.gridCells))}
	if g.gridPending > 0 {
		parts = append(parts, fmt.Sprintf("Cargando %d…", g.gridPending))
	} else {
		parts = append(parts, fmt.Sprintf("%d miniaturas listas", loaded))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d con error", failed))
	}
	g.gridStatus.SetText(strings.Join(parts, " · "))
}

// handleGridKey consume todas las teclas mientras la rejilla está abierta para
// que la página de fondo no se mueva por accidente.
func (g *gutterApp) handleGridKey(event *fyne.KeyEvent) bool {
	if !g.gridOpen {
		return false
	}
	columns := g.gridColumns()
	pageStep := columns * 4
	switch event.Name {
	case fyne.KeyEscape, fyne.KeyM:
		g.hideThumbnailGrid()
	case fyne.KeyReturn, fyne.KeyEnter:
		g.jumpFromGrid()
	case fyne.KeyLeft:
		g.moveGridSelection(-1)
	case fyne.KeyRight:
		g.moveGridSelection(1)
	case fyne.KeyUp:
		g.moveGridSelection(-columns)
	case fyne.KeyDown:
		g.moveGridSelection(columns)
	case fyne.KeyHome:
		g.setGridSelection(0, true)
	case fyne.KeyEnd:
		g.setGridSelection(len(g.gridCells)-1, true)
	case fyne.KeyPageUp:
		g.moveGridSelection(-pageStep)
	case fyne.KeyPageDown:
		g.moveGridSelection(pageStep)
	}
	return true
}

func clampInt(value, low, high int) int {
	if high < low {
		return low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
