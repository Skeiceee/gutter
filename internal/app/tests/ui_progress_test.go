package tests

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"gutter/internal/app"
	"gutter/internal/backend"
)

func TestUpdateControlsShowsRARProgress(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	comic := &backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png"}}
	g.SetComic(comic)
	g.SetPageIndex(0)

	g.SetLoading(true)
	g.SetReadProgress(3, 10)
	g.UpdateControls()

	if !strings.Contains(g.ProgressText(), "RAR sólido 3/10") {
		t.Fatalf("progreso = %q", g.ProgressText())
	}
	if !strings.Contains(g.ProgressText(), "Esc para cancelar") {
		t.Fatalf("el progreso no ofrece cancelar: %q", g.ProgressText())
	}
	if !g.PrevDisabled() || !g.NextDisabled() {
		t.Fatal("los botones deben estar deshabilitados durante la carga")
	}

	g.SetLoading(false)
	g.UpdateControls()
	if !strings.Contains(g.ProgressText(), "Página 1 de 2") {
		t.Fatalf("progreso normal = %q", g.ProgressText())
	}
}

func TestUpdateControlsShowsPageError(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	comic := &backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png", "3.png"}}
	g.SetComic(comic)
	g.SetPageIndex(1)
	g.SetPageFailed(true)

	g.UpdateControls()

	if !strings.Contains(g.ProgressText(), "Página 2 de 3") || !strings.Contains(g.ProgressText(), "error") {
		t.Fatalf("progreso con error = %q", g.ProgressText())
	}
	if g.PrevDisabled() || g.NextDisabled() {
		t.Fatal("la navegación debe seguir disponible tras un error de página")
	}
}

func TestCancelLoadingRestoresShownPage(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	comic := &backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png", "3.png"}}
	g.SetComic(comic)
	g.SetShown(comic, 1, 0, false)
	g.SetPageIndex(2)
	g.SetHalf(0)
	g.SetLoading(true)

	g.CancelLoading()

	if g.Loading() {
		t.Fatal("loading debería quedar en false")
	}
	if g.PageIndex() != 1 {
		t.Fatalf("pageIndex = %d, quiero 1", g.PageIndex())
	}
	if !strings.Contains(g.ProgressText(), "Página 2 de 3") {
		t.Fatalf("progreso restaurado = %q", g.ProgressText())
	}
}
