package tests

// opening_test.go cubre el estado que la interfaz muestra mientras indexa un
// cómic en segundo plano y la cancelación de esa apertura.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"gutter/internal/app"
	"gutter/internal/backend"
)

func TestUpdateControlsShowsOpeningState(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	g.SetComic(&backend.Comic{Path: "comic.cbz", Pages: []string{"1.png", "2.png"}})
	g.SetPageIndex(0)

	g.SetOpening(true)
	g.UpdateControls()

	if !strings.Contains(g.ProgressText(), "Abriendo cómic") {
		t.Fatalf("progreso = %q", g.ProgressText())
	}
	if !strings.Contains(g.ProgressText(), "Esc para cancelar") {
		t.Fatalf("la apertura no ofrece cancelar: %q", g.ProgressText())
	}
	if !g.PrevDisabled() || !g.NextDisabled() {
		t.Fatal("la navegación debe estar deshabilitada mientras se abre")
	}
}

func TestCancelLoadingDiscardsPendingOpen(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	g.SetComic(&backend.Comic{Path: "comic.cbz", Pages: []string{"1.png", "2.png"}})
	g.SetPageIndex(0)

	g.SetOpening(true)
	before := g.OpenSeq()
	g.CancelLoading()

	if g.Opening() {
		t.Fatal("la apertura debe quedar cancelada")
	}
	if g.OpenSeq() == before {
		t.Fatal("cancelar debe invalidar el resultado en vuelo (openSeq)")
	}
	if strings.Contains(g.ProgressText(), "Abriendo cómic") {
		t.Fatalf("el progreso sigue mostrando la apertura: %q", g.ProgressText())
	}
	if g.Loading() {
		t.Fatal("no debe quedar el lector en estado de carga")
	}
}
