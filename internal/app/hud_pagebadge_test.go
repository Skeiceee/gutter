package app

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"gutter/internal/backend"
)

// El indicador de pantalla completa debe actualizarse con cada navegación,
// no quedarse con la página que estaba visible al pulsar F11.
func TestPageBadgeFollowsNavigation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	g := NewGutterApp(app)
	g.comic = &backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png", "3.png"}}
	g.cleanMode = true
	g.pageIndex = 0

	g.updateControls()
	if !g.pageBadge.Visible() {
		t.Fatal("el indicador debe mostrarse en modo limpio")
	}
	if got := g.pageBadge.label.Text; got != "Página 1 / 3" {
		t.Fatalf("indicador = %q, quiero %q", got, "Página 1 / 3")
	}

	g.pageIndex = 2
	g.updateControls()
	if got := g.pageBadge.label.Text; got != "Página 3 / 3" {
		t.Fatalf("indicador tras navegar = %q, quiero %q", got, "Página 3 / 3")
	}
}

// Salir de pantalla completa debe restaurar cleanMode y ocultar el indicador;
// si no, la siguiente pulsación de F11 se interpreta como una salida.
func TestToggleCleanModeRestoresState(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	g := NewGutterApp(app)
	g.comic = &backend.Comic{Path: "test.cbz", Pages: []string{"1.png", "2.png"}}
	g.pageIndex = 0

	g.toggleCleanMode()
	if !g.cleanMode {
		t.Fatal("cleanMode debería activarse al entrar en pantalla completa")
	}
	if !g.pageBadge.Visible() {
		t.Fatal("el indicador debe mostrarse al entrar en pantalla completa")
	}

	g.toggleCleanMode()
	if g.cleanMode {
		t.Fatal("cleanMode debería desactivarse al salir de pantalla completa")
	}
	if g.pageBadge.Visible() {
		t.Fatal("el indicador debe ocultarse al salir de pantalla completa")
	}

	// La siguiente pulsación debe volver a entrar, no quedarse en la salida.
	g.toggleCleanMode()
	if !g.cleanMode {
		t.Fatal("F11 debe volver a entrar en pantalla completa")
	}
}
