package app

// testapi.go expone una superficie mínima y estable para el paquete externo
// internal/app/tests. Go no permite que un _test.go de otra carpeta acceda a
// identificadores sin exportar, así que los tests viven en su propia carpeta y
// hablan con el paquete app a través de estos envoltorios. Solo se exporta lo
// que las pruebas necesitan; la API pública real de la aplicación no cambia.

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/backend"
)

// RecentComic da nombre exportado a la entrada de progreso persistida.
type RecentComic = recentComic

// Constantes y tablas que antes usaban los tests internos.
const (
	MaxRecentComics      = maxRecentComics
	PrefRecentComics     = prefRecentComics
	PrefSortMode         = prefSortMode
	PrefFilterJunk       = prefFilterJunk
	PrefIntegrityCheck   = prefIntegrityCheck
	PrefGridThumbSize    = prefGridThumbSize
	DefaultGridThumbSize = defaultGridThumbSize
)

var (
	DiskLimitLabels = diskLimitLabels
	DiskLimitValues = diskLimitValues
	GridThumbValues = gridThumbValues
)

func LoadRecents(prefs fyne.Preferences) []RecentComic { return loadRecents(prefs) }
func RecentPercent(entry RecentComic) int              { return recentPercent(entry) }
func ClampPage(page, total int) int                    { return clampPage(page, total) }
func ElideLabel(text string, maxRunes int) string      { return elideLabel(text, maxRunes) }
func NormalizeComicPath(path string) string            { return normalizeComicPath(path) }
func IsComicPath(path string) bool                     { return isComicPath(path) }
func DiskLimitLabel(mb int) string                     { return diskLimitLabel(mb) }
func ParseDiskLimit(label string) int                  { return parseDiskLimit(label) }
func FormatBytes(bytes int64) string                   { return formatBytes(bytes) }
func IsRightHalf(half int, manga bool) bool            { return isRightHalf(half, manga) }
func DigitKey(name fyne.KeyName) (string, bool)        { return digitKey(name) }
func GridCellSize(size int) fyne.Size                  { return gridCellSize(size) }
func GridThumbLabel(size int) string                   { return gridThumbLabel(size) }
func ParseGridThumbSize(label string) int              { return parseGridThumbSize(label) }

// TestApp envuelve *gutterApp para los tests externos.
type TestApp struct{ app *gutterApp }

func NewTestApp(a fyne.App) *TestApp { return &TestApp{app: NewGutterApp(a)} }

// NewBareTestApp crea solo el estado que necesitan las pruebas de persistencia,
// sin construir toda la interfaz de Fyne.
func NewBareTestApp(prefs fyne.Preferences) *TestApp {
	return &TestApp{app: &gutterApp{prefs: prefs}}
}

func (t *TestApp) Prefs() fyne.Preferences            { return t.app.prefs }
func (t *TestApp) SetPrefInt(key string, value int)   { t.app.prefs.SetInt(key, value) }
func (t *TestApp) SetPrefString(key, value string)    { t.app.prefs.SetString(key, value) }
func (t *TestApp) SetPrefBool(key string, value bool) { t.app.prefs.SetBool(key, value) }

func (t *TestApp) SetComic(c *backend.Comic) { t.app.comic = c }
func (t *TestApp) Comic() *backend.Comic     { return t.app.comic }

func (t *TestApp) SetRecents(recents []RecentComic) { t.app.recents = recents }
func (t *TestApp) Recents() []RecentComic           { return t.app.recents }
func (t *TestApp) SetDiskCache(c *backend.DiskCache) {
	t.app.diskCache = c
}

func (t *TestApp) SetPageIndex(index int) { t.app.pageIndex = index }
func (t *TestApp) PageIndex() int         { return t.app.pageIndex }

func (t *TestApp) SetHalf(half int) { t.app.half = half }
func (t *TestApp) Half() int        { return t.app.half }

func (t *TestApp) SetManga(manga bool) { t.app.manga = manga }
func (t *TestApp) Manga() bool         { return t.app.manga }

func (t *TestApp) SetSinglePage(single bool) { t.app.singlePage = single }
func (t *TestApp) SinglePage() bool          { return t.app.singlePage }

func (t *TestApp) SetZoomFit(fit bool)    { t.app.zoomFit = fit }
func (t *TestApp) ZoomFit() bool          { return t.app.zoomFit }
func (t *TestApp) SetZoomScale(z float32) { t.app.zoomScale = z }
func (t *TestApp) ZoomScale() float32     { return t.app.zoomScale }

func (t *TestApp) SetCleanMode(clean bool) { t.app.cleanMode = clean }
func (t *TestApp) CleanMode() bool         { return t.app.cleanMode }

func (t *TestApp) SetLoading(loading bool) { t.app.loading = loading }
func (t *TestApp) Loading() bool           { return t.app.loading }

func (t *TestApp) SetReadProgress(scanned, total int64) {
	t.app.readScanned.Store(scanned)
	t.app.readTotal.Store(total)
}

func (t *TestApp) SetPageFailed(failed bool) { t.app.pageFailed = failed }

func (t *TestApp) SetShown(comic *backend.Comic, page, half int, spread bool) {
	t.app.shownComic = comic
	t.app.shownPage = page
	t.app.shownHalf = half
	t.app.shownSpread = spread
}

func (t *TestApp) UpdateControls() { t.app.updateControls() }
func (t *TestApp) ToggleCleanMode() {
	t.app.toggleCleanMode()
}
func (t *TestApp) CancelLoading() { t.app.cancelLoading() }

func (t *TestApp) ProgressText() string {
	if t.app.progress == nil {
		return ""
	}
	return t.app.progress.Text
}

func (t *TestApp) PrevDisabled() bool {
	return t.app.prevButton == nil || t.app.prevButton.Disabled()
}

func (t *TestApp) NextDisabled() bool {
	return t.app.nextButton == nil || t.app.nextButton.Disabled()
}

func (t *TestApp) PageBadgeVisible() bool {
	return t.app.pageBadge != nil && t.app.pageBadge.Visible()
}

func (t *TestApp) PageBadgeText() string {
	if t.app.pageBadge == nil {
		return ""
	}
	return t.app.pageBadge.label.Text
}

func (t *TestApp) UpsertRecent(entry RecentComic) { t.app.upsertRecent(entry) }
func (t *TestApp) RestoreProgress(comic *backend.Comic) {
	t.app.restoreProgress(comic)
}
func (t *TestApp) SaveCurrentProgress() { t.app.saveCurrentProgress() }
func (t *TestApp) OpenOptions() backend.OpenOptions {
	return t.app.openOptions()
}
func (t *TestApp) CacheSummary() string { return t.app.cacheSummary() }

// TestPill envuelve el HUD flotante para comprobar su estado interno.
type TestPill struct{ pill *floatingPill }

func NewTestPill() *TestPill { return &TestPill{pill: newFloatingPill()} }

func (p *TestPill) Root() fyne.CanvasObject { return p.pill.root }
func (p *TestPill) Visible() bool           { return p.pill.Visible() }
func (p *TestPill) SetText(text string)     { p.pill.SetText(text) }
func (p *TestPill) Text() string            { return p.pill.label.Text }
func (p *TestPill) Show()                   { p.pill.Show() }
func (p *TestPill) Hide()                   { p.pill.Hide() }

// TestCell envuelve thumbnailCell y expone solo el estado que verifican los
// tests de la rejilla.
type TestCell struct{ cell *thumbnailCell }

func NewTestCell(index int, name string, size fyne.Size) *TestCell {
	return &TestCell{cell: newThumbnailCell(index, name, size, nil)}
}

func (c *TestCell) NumberText() string           { return c.cell.number.Text }
func (c *TestCell) BorderVisible() bool          { return c.cell.border.Visible() }
func (c *TestCell) Selected() bool               { return c.cell.selected }
func (c *TestCell) Loaded() bool                 { return c.cell.loaded }
func (c *TestCell) Failed() bool                 { return c.cell.failed }
func (c *TestCell) SpreadPage() bool             { return c.cell.spreadPage }
func (c *TestCell) ImageVisible() bool           { return c.cell.image.Visible() }
func (c *TestCell) ErrorVisible() bool           { return c.cell.errorLabel.Visible() }
func (c *TestCell) SpreadVisible() bool          { return c.cell.spread.Visible() }
func (c *TestCell) SetSelected(on bool)          { c.cell.SetSelected(on) }
func (c *TestCell) SetThumbnail(img *image.RGBA) { c.cell.SetThumbnail(img) }
func (c *TestCell) SetError(err error)           { c.cell.SetError(err) }

// InitGrid deja la rejilla en el estado que tendría justo después de pulsar M,
// pero sin lanzar la carga asíncrona. Devuelve el cómic asociado.
func (t *TestApp) InitGrid(pages int) *backend.Comic {
	g := t.app
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
	return comic
}

func (t *TestApp) SetGridThumbSize(size int)           { t.app.gridThumbSize = size }
func (t *TestApp) GridThumbSizePref() int              { return t.app.gridThumbSizePref() }
func (t *TestApp) GridThumbnailRenderSize() int        { return t.app.gridThumbnailRenderSize() }
func (t *TestApp) ShowThumbnailGrid()                  { t.app.showThumbnailGrid() }
func (t *TestApp) HideThumbnailGrid()                  { t.app.hideThumbnailGrid() }
func (t *TestApp) HandleGridKey(e *fyne.KeyEvent) bool { return t.app.handleGridKey(e) }
func (t *TestApp) GridOpen() bool                      { return t.app.gridOpen }
func (t *TestApp) GridSelection() int                  { return t.app.gridSelection }
func (t *TestApp) GridCellCount() int                  { return len(t.app.gridCells) }
func (t *TestApp) GridPending() int                    { return t.app.gridPending }
func (t *TestApp) SetGridPending(n int)                { t.app.gridPending = n }
func (t *TestApp) GridSeqAdd(delta uint64) uint64      { return t.app.gridSeq.Add(delta) }
func (t *TestApp) GridColumns() int                    { return t.app.gridColumns() }
func (t *TestApp) GridRowHeight() float32              { return t.app.gridRowHeight() }
func (t *TestApp) GridVisibleRange() (int, int)        { return t.app.gridVisibleRange() }

func (t *TestApp) GridLoaded(index int) bool { return t.app.gridLoaded[index] }
func (t *TestApp) GridFailed(index int) bool { return t.app.gridFailed[index] }

func (t *TestApp) GridStatusText() string {
	if t.app.gridStatus == nil {
		return ""
	}
	return t.app.gridStatus.Text
}

func (t *TestApp) GridOverlayVisible() bool {
	return t.app.gridOverlay != nil && t.app.gridOverlay.Visible()
}

func (t *TestApp) GridOverlayObjectsNil() bool {
	return t.app.gridOverlay == nil || t.app.gridOverlay.Objects == nil
}

func (t *TestApp) GridStateCleared() bool {
	return t.app.gridCells == nil && t.app.gridScroll == nil && t.app.gridComic == nil
}

func (t *TestApp) GridCellsNil() bool { return t.app.gridCells == nil }

func (t *TestApp) SetGridScrollOffsetY(y float32) {
	if t.app.gridScroll != nil {
		t.app.gridScroll.Offset = fyne.NewPos(0, y)
	}
}

func (t *TestApp) ApplyGridThumbnail(seq uint64, comic *backend.Comic, index int, img *image.RGBA, err error) {
	t.app.applyGridThumbnail(seq, comic, index, img, err)
}
