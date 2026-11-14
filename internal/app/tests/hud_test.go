package tests

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"gutter/internal/app"
)

func TestDigitKey(t *testing.T) {
	cases := map[fyne.KeyName]string{
		fyne.Key0: "0",
		fyne.Key1: "1",
		fyne.Key2: "2",
		fyne.Key3: "3",
		fyne.Key4: "4",
		fyne.Key5: "5",
		fyne.Key6: "6",
		fyne.Key7: "7",
		fyne.Key8: "8",
		fyne.Key9: "9",
	}

	for key, want := range cases {
		if got, ok := app.DigitKey(key); !ok || got != want {
			t.Errorf("DigitKey(%q) = %q, %v; quiero %q, true", key, got, ok, want)
		}
	}

	// Un teclado numérico envía KeyEnter/KeyKP igual, pero letras y flechas
	// nunca deben interpretarse como dígitos.
	for _, key := range []fyne.KeyName{fyne.KeyA, fyne.KeyRight, fyne.KeyEscape, fyne.KeyEnter} {
		if got, ok := app.DigitKey(key); ok {
			t.Errorf("DigitKey(%q) = %q, true; quiero que falle", key, got)
		}
	}
}

// TestFloatingPillEsContenedorReal protege el detalle de pintado de Fyne 2.8:
// el recorrido solo desciende a los hijos de un *fyne.Container exacto, así que
// el HUD debe exponer ese contenedor y no un tipo que lo embeba.
func TestFloatingPillEsContenedorReal(t *testing.T) {
	test.NewApp()

	pill := app.NewTestPill()
	if _, ok := pill.Root().(*fyne.Container); !ok {
		t.Fatalf("pill.root no es *fyne.Container, es %T", pill.Root())
	}
	if pill.Visible() {
		t.Fatal("el HUD debe nacer oculto")
	}

	pill.SetText("Página 7 / 42")
	pill.Show()
	if !pill.Visible() {
		t.Fatal("Show debería hacer visible el HUD")
	}
	if got := pill.Text(); got != "Página 7 / 42" {
		t.Fatalf("texto = %q, quiero %q", got, "Página 7 / 42")
	}

	pill.Hide()
	if pill.Visible() {
		t.Fatal("Hide debería ocultar el HUD")
	}
}
