// library.go guarda el progreso de lectura, mantiene la lista de cómics
// recientes y construye la pantalla de bienvenida.
//
// Toda la persistencia vive en fyne.Preferences, así que no hay archivos de
// configuración propios ni rutas que mantener. El progreso se guarda por cómic
// (ruta canónica) y la lista se ordena por la última vez que se abrió.
package app

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/backend"
)

const (
	prefRecentComics = "reader.recent"
	prefWindowWidth  = "ui.window.width"
	prefWindowHeight = "ui.window.height"

	// La lista es deliberadamente corta: cabe en la pantalla de bienvenida y
	// mantiene el JSON de preferencias pequeño.
	maxRecentComics = 15
)

// recentComic es a la vez una entrada de "Recientes" y el progreso guardado de
// un cómic. Page y Half son índices en orden de lectura.
type recentComic struct {
	Path       string  `json:"path"`
	Page       int     `json:"page"`
	Half       int     `json:"half"`
	Total      int     `json:"total"`
	Manga      bool    `json:"manga"`
	SinglePage bool    `json:"single_page"`
	ZoomFit    bool    `json:"zoom_fit"`
	ZoomScale  float32 `json:"zoom_scale"`
	UpdatedAt  int64   `json:"updated_at"`
}

// loadRecents lee, valida y ordena la lista guardada. Una entrada corrupta no
// debe impedir abrir la aplicación: se descarta sin más.
func loadRecents(prefs fyne.Preferences) []recentComic {
	raw := strings.TrimSpace(prefs.String(prefRecentComics))
	if raw == "" {
		return nil
	}

	var stored []recentComic
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil
	}

	seen := make(map[string]struct{}, len(stored))
	out := make([]recentComic, 0, len(stored))
	for _, entry := range stored {
		entry.Path = normalizeComicPath(entry.Path)
		if entry.Path == "" {
			continue
		}
		key := pathKey(entry.Path)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}

		if entry.Page < 0 {
			entry.Page = 0
		}
		if entry.Total < 0 {
			entry.Total = 0
		}
		if entry.Total > 0 && entry.Page >= entry.Total {
			entry.Page = entry.Total - 1
		}
		if entry.Half < 0 || entry.Half > 1 {
			entry.Half = 0
		}
		if entry.ZoomScale <= 0 {
			entry.ZoomScale = 1
		}
		out = append(out, entry)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	if len(out) > maxRecentComics {
		out = out[:maxRecentComics]
	}
	return out
}

// saveRecents serializa la lista completa. Es un único valor de preferencias:
// pequeño, atómico para Fyne y fácil de inspeccionar.
func (g *gutterApp) saveRecents() {
	if g.prefs == nil {
		return
	}
	data, err := json.Marshal(g.recents)
	if err != nil {
		return
	}
	g.prefs.SetString(prefRecentComics, string(data))
}

// upsertRecent coloca el cómic al principio de la lista y actualiza su entrada
// si ya existía. No duplica rutas aunque cambien mayúsculas en Windows.
func (g *gutterApp) upsertRecent(entry recentComic) {
	entry.Path = normalizeComicPath(entry.Path)
	if entry.Path == "" {
		return
	}
	entry.UpdatedAt = time.Now().Unix()

	key := pathKey(entry.Path)
	updated := make([]recentComic, 0, len(g.recents)+1)
	updated = append(updated, entry)
	for _, old := range g.recents {
		if pathKey(old.Path) == key {
			continue
		}
		updated = append(updated, old)
	}
	if len(updated) > maxRecentComics {
		updated = updated[:maxRecentComics]
	}

	g.recents = updated
	g.saveRecents()
	if g.comic == nil {
		g.refreshWelcome()
	}
}

func (g *gutterApp) findRecent(comicPath string) (recentComic, bool) {
	key := pathKey(comicPath)
	for _, entry := range g.recents {
		if pathKey(entry.Path) == key {
			return entry, true
		}
	}
	return recentComic{}, false
}

// saveCurrentProgress captura página, mitad, sentido, modo y zoom. Se llama al
// mostrar una página y al cerrar la ventana; así no se pierde nada importante
// aunque el cierre sea inmediato.
func (g *gutterApp) saveCurrentProgress() {
	if g.comic == nil || len(g.comic.Pages) == 0 {
		return
	}
	g.upsertRecent(recentComic{
		Path:       g.comic.Path,
		Page:       g.pageIndex,
		Half:       g.half,
		Total:      len(g.comic.Pages),
		Manga:      g.manga,
		SinglePage: g.singlePage,
		ZoomFit:    g.zoomFit,
		ZoomScale:  g.zoomScale,
	})
}

// restoreProgress aplica la última sesión del cómic. Si no hay entrada previa,
// el cómic empieza en la primera página y en modo ajuste al ancho.
func (g *gutterApp) restoreProgress(comic *backend.Comic) {
	g.pageIndex = 0
	g.half = 0
	g.zoomFit = true
	g.zoomScale = 1

	entry, ok := g.findRecent(comic.Path)
	if !ok || len(comic.Pages) == 0 {
		g.syncReadingControls()
		return
	}

	g.pageIndex = clampPage(entry.Page, len(comic.Pages))
	if entry.Half == 1 {
		g.half = 1
	}
	g.manga = entry.Manga
	g.singlePage = entry.SinglePage
	g.zoomFit = entry.ZoomFit
	if entry.ZoomScale > 0 {
		g.zoomScale = clampZoom(entry.ZoomScale)
	}
	g.syncReadingControls()
}

func clampPage(page, total int) int {
	if page < 0 {
		return 0
	}
	if page >= total {
		return total - 1
	}
	return page
}

// normalizeComicPath convierte cualquier ruta en una clave estable y absoluta.
// Es la misma que se guarda en Recientes y la que usa el motor para abrir.
func normalizeComicPath(comicPath string) string {
	comicPath = strings.TrimSpace(comicPath)
	if comicPath == "" {
		return ""
	}
	if abs, err := filepath.Abs(comicPath); err == nil {
		comicPath = abs
	}
	return filepath.Clean(comicPath)
}

// pathKey compara rutas sin distinguir mayúsculas en Windows.
func pathKey(comicPath string) string {
	comicPath = filepath.Clean(comicPath)
	if runtime.GOOS == "windows" {
		return strings.ToLower(comicPath)
	}
	return comicPath
}

func recentName(entry recentComic) string {
	return filepath.Base(entry.Path)
}

// elideLabel acorta nombres muy largos para que un botón no ensanche toda la
// pantalla de bienvenida. El detalle completo sigue visible en el diálogo de
// recientes y en la ruta guardada.
func elideLabel(text string, maxRunes int) string {
	runes := []rune(text)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return text
	}
	if maxRunes == 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

func recentPercent(entry recentComic) int {
	if entry.Total <= 0 {
		return 0
	}
	percent := (entry.Page + 1) * 100 / entry.Total
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func recentDetail(entry recentComic) string {
	if entry.Total <= 0 {
		return "Progreso desconocido"
	}
	return fmt.Sprintf("Página %d de %d · %d %%", entry.Page+1, entry.Total, recentPercent(entry))
}

// refreshWelcome reconstruye la pantalla de bienvenida. Se usa cuando cambia la
// lista de recientes y al arrancar sin cómic.
func (g *gutterApp) refreshWelcome() {
	if g.emptyState == nil {
		return
	}
	g.emptyState.RemoveAll()
	g.emptyState.Add(g.buildWelcome())
	g.emptyState.Refresh()
}

// buildWelcome es la portada de la aplicación: marca, acción principal y
// recientes. El contenido va dentro de un Scroll para que siga siendo usable
// con una lista larga o una ventana baja.
func (g *gutterApp) buildWelcome() fyne.CanvasObject {
	logo := canvas.NewImageFromResource(g.logo)
	logo.FillMode = canvas.ImageFillContain
	logo.ScaleMode = canvas.ImageScaleSmooth
	logo.SetMinSize(fyne.NewSize(320, 120))

	hint := widget.NewLabelWithStyle(
		"Abre un cómic .cbr o .cbz, o arrástralo aquí",
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)
	hint.Importance = widget.LowImportance

	openButton := widget.NewButtonWithIcon("Abrir cómic", theme.FolderOpenIcon(), g.openFile)
	openButton.Importance = widget.HighImportance

	body := container.NewVBox(
		container.NewCenter(logo),
		container.NewCenter(hint),
		container.NewCenter(openButton),
	)

	if len(g.recents) > 0 {
		latest := g.recents[0]
		continueButton := widget.NewButtonWithIcon(
			fmt.Sprintf("Continuar «%s»", elideLabel(recentName(latest), 48)),
			theme.MediaPlayIcon(),
			func() { g.LoadComic(latest.Path) },
		)
		continueButton.Importance = widget.HighImportance

		detail := widget.NewLabelWithStyle(recentDetail(latest), fyne.TextAlignCenter, fyne.TextStyle{})
		detail.Importance = widget.LowImportance

		body.Add(layout.NewSpacer())
		body.Add(container.NewCenter(continueButton))
		body.Add(container.NewCenter(detail))

		if len(g.recents) > 1 {
			heading := widget.NewLabelWithStyle("Recientes", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			body.Add(heading)
			for _, entry := range g.recents[1:] {
				body.Add(g.recentButton(entry))
			}
		}

		clearButton := widget.NewButton("Limpiar recientes", g.confirmClearRecents)
		clearButton.Importance = widget.LowImportance
		body.Add(container.NewCenter(clearButton))
	}

	body.Add(layout.NewSpacer())
	centered := container.NewBorder(layout.NewSpacer(), layout.NewSpacer(), nil, nil, body)
	padded := container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), centered)
	return container.NewVScroll(padded)
}

func (g *gutterApp) recentButton(entry recentComic) *widget.Button {
	button := widget.NewButtonWithIcon(
		fmt.Sprintf("%s  ·  %s", elideLabel(recentName(entry), 48), recentDetail(entry)),
		theme.FileIcon(),
		func() { g.LoadComic(entry.Path) },
	)
	button.Alignment = widget.ButtonAlignLeading
	button.Importance = widget.LowImportance
	return button
}

// showRecents permite cambiar de cómic sin volver a la portada.
func (g *gutterApp) showRecents() {
	if len(g.recents) == 0 {
		dialog.ShowInformation("Recientes", "Todavía no has abierto ningún cómic.", g.window)
		return
	}

	var listDialog *dialog.CustomDialog
	body := container.NewVBox()
	for _, entry := range g.recents {
		entry := entry
		button := widget.NewButtonWithIcon(
			fmt.Sprintf("%s  ·  %s", elideLabel(recentName(entry), 48), recentDetail(entry)),
			theme.FileIcon(),
			func() {
				if listDialog != nil {
					listDialog.Hide()
				}
				g.LoadComic(entry.Path)
			},
		)
		button.Alignment = widget.ButtonAlignLeading
		button.Importance = widget.LowImportance
		body.Add(button)
	}

	clearButton := widget.NewButton("Limpiar recientes", func() {
		if listDialog != nil {
			listDialog.Hide()
		}
		g.confirmClearRecents()
	})
	clearButton.Importance = widget.LowImportance
	body.Add(container.NewCenter(clearButton))

	scroll := container.NewVScroll(body)
	scroll.SetMinSize(fyne.NewSize(480, 320))

	listDialog = dialog.NewCustom("Recientes", "Cerrar", scroll, g.window)
	listDialog.Resize(fyne.NewSize(560, 460))
	g.showDialog(listDialog)
}

// saveWindowSize guarda el tamaño de la ventana en modo normal. Si se cierra
// mientras está a pantalla completa, conserva el último tamaño de ventana.
func (g *gutterApp) saveWindowSize() {
	if g.prefs == nil {
		return
	}
	size := g.lastWindowedSize
	if !g.cleanMode && !g.window.FullScreen() {
		if current := g.window.Canvas().Size(); current.Width > 0 && current.Height > 0 {
			size = current
		}
	}
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	g.prefs.SetFloat(prefWindowWidth, float64(size.Width))
	g.prefs.SetFloat(prefWindowHeight, float64(size.Height))
}

func (g *gutterApp) confirmClearRecents() {
	d := dialog.NewConfirm(
		"Limpiar recientes",
		"Se quitarán todos los cómics de la lista y su progreso guardado. ¿Continuar?",
		func(confirmed bool) {
			if !confirmed {
				return
			}
			g.recents = nil
			g.saveRecents()
			g.refreshWelcome()
		},
		g.window,
	)
	g.showDialog(d)
}
