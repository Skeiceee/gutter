package app

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"gutter/internal/backend"
)

func TestUpdateControlsShowsRARProgress(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	g := NewGutterApp(app)
	comic := &backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png"}}
	g.reader.comic = comic
	g.reader.pageIndex = 0

	g.reader.loading = true
	g.readScanned.Store(3)
	g.readTotal.Store(10)
	g.updateControls()

	if !strings.Contains(g.progress.Text, "RAR sólido 3/10") {
		t.Fatalf("progreso = %q", g.progress.Text)
	}
	if !strings.Contains(g.progress.Text, "Esc para cancelar") {
		t.Fatalf("el progreso no ofrece cancelar: %q", g.progress.Text)
	}
	if !g.prevButton.Disabled() || !g.nextButton.Disabled() {
		t.Fatal("los botones deben estar deshabilitados durante la carga")
	}

	g.reader.loading = false
	g.updateControls()
	if !strings.Contains(g.progress.Text, "Página 1 de 2") {
		t.Fatalf("progreso normal = %q", g.progress.Text)
	}
}

func TestUpdateControlsShowsPageError(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	g := NewGutterApp(app)
	comic := &backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png", "3.png"}}
	g.reader.comic = comic
	g.reader.pageIndex = 1
	g.pageFailed = true

	g.updateControls()

	if !strings.Contains(g.progress.Text, "Página 2 de 3") || !strings.Contains(g.progress.Text, "error") {
		t.Fatalf("progreso con error = %q", g.progress.Text)
	}
	if g.prevButton.Disabled() || g.nextButton.Disabled() {
		t.Fatal("la navegación debe seguir disponible tras un error de página")
	}
}

func TestCancelLoadingRestoresShownPage(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	g := NewGutterApp(app)
	comic := &backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png", "3.png"}}
	g.reader.comic = comic
	g.reader.shownComic = comic
	g.reader.shownPage = 1
	g.reader.shownHalf = 0
	g.reader.shownSpread = false
	g.reader.pageIndex = 2
	g.reader.half = 0
	g.reader.loading = true

	g.cancelLoading()

	if g.reader.loading {
		t.Fatal("loading debería quedar en false")
	}
	if g.reader.pageIndex != 1 {
		t.Fatalf("pageIndex = %d, quiero 1", g.reader.pageIndex)
	}
	if !strings.Contains(g.progress.Text, "Página 2 de 3") {
		t.Fatalf("progreso restaurado = %q", g.progress.Text)
	}
}
