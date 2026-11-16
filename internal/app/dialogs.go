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

// openFile abre el diálogo para seleccionar un cómic .cbr/.cbz. Pasa por
// showDialog para que pulsar Ctrl+O varias veces no apile ventanas modales.
func (g *gutterApp) openFile() {
	filter := storage.NewExtensionFileFilter([]string{".cbr", ".cbz"})
	d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		g.LoadComic(uc.URI().Path())
	}, g.window)
	d.SetFilter(filter)
	d.Resize(fyne.NewSize(880, 680))
	g.showDialog(d)
}

// loadComic abre un archivo .cbr/.cbz y lo prepara para lectura. Si pageName
// no está vacío, intenta colocarse en esa página después de restaurar el
// progreso; es lo que usa el cambio de opciones de orden.
//
// El indexado se hace en segundo plano: un archivo grande puede tardar y no
// debe congelar la ventana. Mientras tanto se mantiene el cómic anterior en
// pantalla y la barra inferior muestra "Abriendo cómic…". Cada apertura lleva
// un número de secuencia: si el usuario abre otro archivo antes de que termine,
// el resultado viejo se descarta y se cierra el cómic que ya no se usa.
func (g *gutterApp) loadComic(comicPath, pageName string) {
	comicPath = normalizeComicPath(comicPath)
	if comicPath == "" {
		return
	}

	g.hideThumbnailGrid()
	g.cancelPageWork()
	g.cancelTranslation()
	g.clearPageError()

	// La carga de página anterior quedó cancelada y su goroutine ya no va a
	// actualizar el estado: se descarta el marcador de "cargando".
	g.reader.loading = false
	g.readScanned.Store(0)
	g.readTotal.Store(0)

	seq := g.openSeq.Add(1)
	g.opening = true
	g.updateControls()

	opts := g.openOptions()
	go func() {
		comic, err := backend.OpenComicWithOptions(comicPath, opts)
		fyne.Do(func() {
			// Otra apertura más reciente (o el cierre de la ventana) invalida
			// este resultado.
			if seq != g.openSeq.Load() || g.baseContext().Err() != nil {
				if comic != nil {
					// Cerrar puede esperar a la caché en disco: fuera del hilo
					// de UI.
					go func() { _ = comic.Close() }()
				}
				return
			}
			g.finishOpening(comic, err, pageName)
		})
	}()
}

// finishOpening instala el cómic ya indexado. Se ejecuta en el hilo de UI,
// después de comprobar que la apertura sigue siendo la vigente.
func (g *gutterApp) finishOpening(comic *backend.Comic, err error, pageName string) {
	g.opening = false

	if err != nil {
		g.updateControls()
		g.hideActiveDialog()
		dialog.ShowError(fmt.Errorf("No se pudo abrir el cómic.\n\n%w", err), g.window)
		return
	}

	// Se instala el nuevo cómic antes de cerrar el actual: así, si algo falla
	// al dibujar la primera página, el usuario conserva lo que ya tenía.
	oldComic := g.reader.comic
	g.reader.comic = comic

	g.restoreProgress(comic)
	if pageName != "" {
		if index := indexOfPage(comic.Pages, pageName); index >= 0 {
			g.reader.pageIndex = index
			g.reader.half = 0
		}
	}

	if oldComic != nil && oldComic != comic {
		// Cerrar puede esperar a las escrituras de caché y a un prefetch en
		// vuelo: se hace fuera del hilo de UI.
		go func() { _ = oldComic.Close() }()
	}

	g.showPage()

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

// cancelLoading cancela la apertura o la carga en curso. Si estaba abriendo un
// cómic, el resultado en vuelo se descarta y se conserva el que ya había.
func (g *gutterApp) cancelLoading() {
	if g.opening {
		g.openSeq.Add(1)
		g.opening = false
		g.updateControls()
		g.unfocus()
		return
	}
	if !g.reader.loading {
		return
	}
	g.cancelPageWork()
	g.reader.loading = false
	g.readScanned.Store(0)
	g.readTotal.Store(0)
	g.clearPageError()
	if g.pageImage.Image != nil {
		g.scroll.Show()
	}

	g.reader.restoreShown()
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
