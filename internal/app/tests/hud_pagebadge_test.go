package tests

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"gutter/internal/app"
	"gutter/internal/backend"
)

// El indicador de pantalla completa debe actualizarse con cada navegación,
// no quedarse con la página que estaba visible al pulsar F11.
func TestPageBadgeFollowsNavigation(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	g.SetComic(&backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png", "3.png"}})
	g.SetCleanMode(true)
	g.SetPageIndex(0)

	g.UpdateControls()
	if !g.PageBadgeVisible() {
		t.Fatal("el indicador debe mostrarse en modo limpio")
	}
	if got := g.PageBadgeText(); got != "Página 1 / 3" {
		t.Fatalf("indicador = %q, quiero %q", got, "Página 1 / 3")
	}

	g.SetPageIndex(2)
	g.UpdateControls()
	if got := g.PageBadgeText(); got != "Página 3 / 3" {
		t.Fatalf("indicador tras navegar = %q, quiero %q", got, "Página 3 / 3")
	}
}

// Salir de pantalla completa debe restaurar cleanMode y ocultar el indicador;
// si no, la siguiente pulsación de F11 se interpreta como una salida.
func TestToggleCleanModeRestoresState(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	g.SetComic(&backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png"}})
	g.SetPageIndex(0)

	g.ToggleCleanMode()
	if !g.CleanMode() {
		t.Fatal("cleanMode debería activarse al entrar en pantalla completa")
	}
	if !g.PageBadgeVisible() {
		t.Fatal("el indicador debe mostrarse al entrar en pantalla completa")
	}

	g.ToggleCleanMode()
	if g.CleanMode() {
		t.Fatal("cleanMode debería desactivarse al salir de pantalla completa")
	}
	if g.PageBadgeVisible() {
		t.Fatal("el indicador debe ocultarse al salir de pantalla completa")
	}

	// La siguiente pulsación debe volver a entrar, no quedarse en la salida.
	g.ToggleCleanMode()
	if !g.CleanMode() {
		t.Fatal("F11 debe volver a entrar en pantalla completa")
	}
}
