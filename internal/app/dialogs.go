package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"gutter/internal/backend"
)

// showDialog garantiza que solo haya un diálogo modal abierto a la vez. Si ya
// hay uno, lo cierra antes de mostrar el nuevo: evita que pulsar F1, Ctrl+O,
// etc. varias veces apile ventanas sobre la misma.
func (g *gutterApp) showDialog(d dialog.Dialog) {
	if g.activeDialog != nil && g.activeDialog != d {
		g.activeDialog.Hide()
		g.activeDialog = nil
	}
	g.activeDialog = d
	d.SetOnClosed(func() {
		if g.activeDialog == d {
			g.activeDialog = nil
		}
	})
	d.Show()
}

// hideActiveDialog cierra el diálogo modal actual, si lo hay. Se usa antes de
// mostrar avisos rápidos (ShowError/ShowInformation), que no pasan por showDialog.
func (g *gutterApp) hideActiveDialog() {
	if g.activeDialog != nil {
		g.activeDialog.Hide()
		g.activeDialog = nil
	}
}

// openFile abre el diálogo para seleccionar un cómic .cbr/.cbz.
func (g *gutterApp) openFile() {
	filter := storage.NewExtensionFileFilter([]string{".cbr", ".cbz"})
	dialog := dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		g.LoadComic(uc.URI().Path())
	}, g.window)
	dialog.SetFilter(filter)
	dialog.Resize(fyne.NewSize(880, 680))
	dialog.Show()
}

// loadComic abre un archivo .cbr/.cbz y lo prepara para lectura. Si pageName
// no está vacío, intenta colocarse en esa página después de restaurar el
// progreso; es lo que usa el cambio de opciones de orden.
func (g *gutterApp) loadComic(comicPath, pageName string) {
	if comicPath == "" {
		return
	}

	g.cancelPageWork()
	g.clearPageError()
	seq := g.pageSeq.Add(1)

	// Se abre el nuevo cómic antes de cerrar el actual: si el archivo está
	// dañado, el usuario conserva la lectura que ya tenía en pantalla.
	comic, err := backend.OpenComicWithOptions(comicPath, g.openOptions())
	if err != nil {
		g.reportOpenError(seq, err)
		return
	}

	oldComic := g.comic
	g.comic = comic

	g.restoreProgress(comic)
	if pageName != "" {
		if index := indexOfPage(comic.Pages, pageName); index >= 0 {
			g.pageIndex = index
			g.half = 0
		}
	}

	if oldComic != nil {
		_ = oldComic.Close()
	}

	g.showPage()
	g.window.SetTitle(fmt.Sprintf("%s — %s", appTitle, filepath.Base(comicPath)))

	if problems := comic.Problems(); len(problems) > 0 {
		g.showIntegrityWarning(problems)
	}
}

// LoadComic abre un cómic conservando el progreso guardado.
func (g *gutterApp) LoadComic(comicPath string) {
	g.loadComic(comicPath, "")
}

func indexOfPage(pages []string, name string) int {
	for index, page := range pages {
		if page == name {
			return index
		}
	}
	return -1
}

// cancelPageWork detiene la carga actual y el prefetch.
func (g *gutterApp) cancelPageWork() {
	if g.pageCancel != nil {
		g.pageCancel()
		g.pageCancel = nil
	}
	if g.prefetchCancel != nil {
		g.prefetchCancel()
		g.prefetchCancel = nil
	}
}

// cancelLoading cancela la carga en curso y restaura la página mostrada.
func (g *gutterApp) cancelLoading() {
	if !g.loading {
		return
	}
	g.cancelPageWork()
	g.loading = false
	g.readScanned.Store(0)
	g.readTotal.Store(0)
	g.clearPageError()
	if g.pageImage.Image != nil {
		g.scroll.Show()
	}

	if g.shownComic == g.comic {
		g.pageIndex = g.shownPage
		g.half = g.shownHalf
		g.spread = g.shownSpread
	} else {
		g.pageIndex = 0
		g.half = 0
		g.spread = false
	}
	g.updateControls()
	g.unfocus()
}

// isComicPath verifica si una ruta corresponde a un archivo de cómic soportado.
func isComicPath(comicPath string) bool {
	ext := strings.ToLower(filepath.Ext(comicPath))
	return ext == ".cbr" || ext == ".cbz"
}

// loadBrandResource carga el logotipo de Gutter desde los SVG embebidos o disco.
func loadBrandResource(candidates ...string) fyne.Resource {
	for _, rel := range candidates {
		if data, err := os.ReadFile(filepath.FromSlash(rel)); err == nil && len(data) > 0 {
			return fyne.NewStaticResource(filepath.Base(rel), data)
		}
		if data, err := brandFiles.ReadFile(rel); err == nil && len(data) > 0 {
			return fyne.NewStaticResource(filepath.Base(rel), data)
		}
	}
	return theme.FyneLogo()
}
