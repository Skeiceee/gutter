package app

// bookmarks.go guarda marcadores con nombre y nota corta por página. Viven en
// fyne.Preferences como una lista JSON, igual que Recientes: no hay archivos
// propios que mantener. Un cómic puede tener un marcador por página.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	prefBookmarks = "reader.bookmarks"

	// Límite global para que el JSON de preferencias no crezca sin control.
	maxBookmarks = 2000
	// Cuántos marcadores del cómic actual se listan directamente en el menú.
	maxBookmarkMenuItems = 9
)

type bookmark struct {
	Path      string `json:"path"`
	Page      int    `json:"page"`
	Name      string `json:"name"`
	Note      string `json:"note"`
	CreatedAt int64  `json:"created_at"`
}

// loadBookmarks lee y valida la lista guardada. Las entradas incompletas o
// duplicadas se descartan sin impedir el arranque.
func loadBookmarks(prefs fyne.Preferences) []bookmark {
	if prefs == nil {
		return nil
	}
	raw := strings.TrimSpace(prefs.String(prefBookmarks))
	if raw == "" {
		return nil
	}

	var stored []bookmark
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil
	}

	seen := make(map[string]struct{}, len(stored))
	out := make([]bookmark, 0, len(stored))
	for _, entry := range stored {
		entry.Path = normalizeComicPath(entry.Path)
		if entry.Path == "" || entry.Page < 0 {
			continue
		}
		entry.Name = strings.TrimSpace(entry.Name)
		if entry.Name == "" {
			entry.Name = fmt.Sprintf("Página %d", entry.Page+1)
		}
		key := fmt.Sprintf("%s|%d", pathKey(entry.Path), entry.Page)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, entry)
	}
	if len(out) > maxBookmarks {
		out = out[:maxBookmarks]
	}
	return out
}

func (g *gutterApp) saveBookmarks() {
	if g.prefs == nil {
		return
	}
	data, err := json.Marshal(g.bookmarks)
	if err != nil {
		return
	}
	g.prefs.SetString(prefBookmarks, string(data))
}

// bookmarksFor devuelve los marcadores de un cómic ordenados por página.
func (g *gutterApp) bookmarksFor(comicPath string) []bookmark {
	key := pathKey(comicPath)
	var out []bookmark
	for _, entry := range g.bookmarks {
		if pathKey(entry.Path) == key {
			out = append(out, entry)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Page < out[j].Page })
	return out
}

// bookmarkAt busca el marcador de una página concreta del cómic abierto.
func (g *gutterApp) bookmarkAt(page int) (bookmark, bool) {
	if g.reader.comic == nil {
		return bookmark{}, false
	}
	key := pathKey(g.reader.comic.Path)
	for _, entry := range g.bookmarks {
		if entry.Page == page && pathKey(entry.Path) == key {
			return entry, true
		}
	}
	return bookmark{}, false
}

// upsertBookmark añade o reemplaza el marcador de la misma página.
func (g *gutterApp) upsertBookmark(entry bookmark) {
	entry.Path = normalizeComicPath(entry.Path)
	if entry.Path == "" {
		return
	}
	key := pathKey(entry.Path)
	updated := make([]bookmark, 0, len(g.bookmarks)+1)
	replaced := false
	for _, old := range g.bookmarks {
		if old.Page == entry.Page && pathKey(old.Path) == key {
			if !replaced {
				updated = append(updated, entry)
				replaced = true
			}
			continue
		}
		updated = append(updated, old)
	}
	if !replaced {
		updated = append(updated, entry)
	}
	if len(updated) > maxBookmarks {
		updated = updated[len(updated)-maxBookmarks:]
	}
	g.bookmarks = updated
	g.saveBookmarks()
	g.refreshMenu()
}

func (g *gutterApp) removeBookmark(comicPath string, page int) {
	key := pathKey(comicPath)
	updated := g.bookmarks[:0]
	for _, old := range g.bookmarks {
		if old.Page == page && pathKey(old.Path) == key {
			continue
		}
		updated = append(updated, old)
	}
	g.bookmarks = updated
	g.saveBookmarks()
	g.refreshMenu()
}

// openBookmarkDialog crea o edita el marcador de la página actual.
func (g *gutterApp) openBookmarkDialog() {
	if g.reader.comic == nil || g.reader.loading || g.opening {
		dialog.ShowInformation("Marcadores", "Abre un cómic para marcar una página.", g.window)
		return
	}
	if len(g.reader.comic.Pages) == 0 {
		return
	}

	page := g.reader.pageIndex
	existing, editing := g.bookmarkAt(page)

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder(fmt.Sprintf("Página %d", page+1))
	noteEntry := widget.NewMultiLineEntry()
	noteEntry.SetPlaceHolder("Nota corta (opcional)")
	noteEntry.Wrapping = fyne.TextWrapWord
	noteEntry.SetMinRowsVisible(2)
	if editing {
		nameEntry.SetText(existing.Name)
		noteEntry.SetText(existing.Note)
	}

	title := "Nuevo marcador"
	if editing {
		title = "Editar marcador"
	}
	form := widget.NewForm(
		widget.NewFormItem("Nombre", nameEntry),
		widget.NewFormItem("Nota", noteEntry),
	)
	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Página %d de %d", page+1, len(g.reader.comic.Pages))),
		form,
	)

	comicPath := g.reader.comic.Path
	d := dialog.NewCustomConfirm(title, "Guardar", "Cancelar", content, func(saved bool) {
		if !saved {
			return
		}
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			name = fmt.Sprintf("Página %d", page+1)
		}
		g.upsertBookmark(bookmark{
			Path:      comicPath,
			Page:      page,
			Name:      name,
			Note:      strings.TrimSpace(noteEntry.Text),
			CreatedAt: time.Now().Unix(),
		})
	}, g.window)
	d.Resize(fyne.NewSize(460, 320))
	g.showDialog(d)
	g.unfocus()
}

// showBookmarkList lista los marcadores del cómic abierto con acciones para
// saltar a ellos o borrarlos.
func (g *gutterApp) showBookmarkList() {
	if g.reader.comic == nil {
		dialog.ShowInformation("Marcadores", "Abre un cómic para ver sus marcadores.", g.window)
		return
	}
	comicPath := g.reader.comic.Path
	entries := g.bookmarksFor(comicPath)

	rows := container.NewVBox()
	if len(entries) == 0 {
		rows.Add(widget.NewLabel("Este cómic todavía no tiene marcadores.\nPulsa B para marcar la página actual."))
	}
	for _, entry := range entries {
		entry := entry
		jump := widget.NewButtonWithIcon("Ir", theme.NavigateNextIcon(), func() {
			g.hideActiveDialog()
			g.goToPage(entry.Page)
		})
		jump.Importance = widget.HighImportance
		remove := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			g.removeBookmark(comicPath, entry.Page)
			g.hideActiveDialog()
			g.showBookmarkList()
		})
		remove.Importance = widget.LowImportance

		title := widget.NewLabelWithStyle(
			fmt.Sprintf("Página %d · %s", entry.Page+1, entry.Name),
			fyne.TextAlignLeading,
			fyne.TextStyle{Bold: true},
		)
		title.Truncation = fyne.TextTruncateEllipsis
		labels := []fyne.CanvasObject{title}
		if entry.Note != "" {
			note := widget.NewLabel(entry.Note)
			note.Wrapping = fyne.TextWrapWord
			note.Importance = widget.LowImportance
			labels = append(labels, note)
		}
		text := container.New(layout.NewVBoxLayout(), labels...)
		rows.Add(container.NewBorder(nil, nil, nil, container.NewHBox(jump, remove), text))
		rows.Add(widget.NewSeparator())
	}

	scroll := container.NewVScroll(rows)
	scroll.SetMinSize(fyne.NewSize(480, 300))
	d := dialog.NewCustom("Marcadores", "Cerrar", scroll, g.window)
	d.Resize(fyne.NewSize(560, 440))
	g.showDialog(d)
}

// bookmarkMenuItems genera las entradas de menú para los marcadores del cómic
// abierto. Se limita el número para no desbordar el menú.
func (g *gutterApp) bookmarkMenuItems() []*fyne.MenuItem {
	if g.reader.comic == nil {
		return nil
	}
	entries := g.bookmarksFor(g.reader.comic.Path)
	if len(entries) == 0 {
		return nil
	}
	items := make([]*fyne.MenuItem, 0, maxBookmarkMenuItems+1)
	for i, entry := range entries {
		if i >= maxBookmarkMenuItems {
			break
		}
		page := entry.Page
		items = append(items, fyne.NewMenuItem(
			fmt.Sprintf("Página %d · %s", page+1, entry.Name),
			func() { g.goToPage(page) },
		))
	}
	if len(entries) > maxBookmarkMenuItems {
		items = append(items, fyne.NewMenuItem(
			fmt.Sprintf("… y %d más", len(entries)-maxBookmarkMenuItems),
			g.showBookmarkList,
		))
	}
	return items
}
