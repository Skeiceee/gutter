package app

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gutter/internal/ocr"
)

// TestManualTranslation es una prueba de humo opcional del flujo completo:
// OCR + traducción + capa visual. Requiere red y una imagen con texto.
func TestManualTranslation(t *testing.T) {
	if os.Getenv("GUTTER_TEST_TRANSLATION") == "" {
		t.Skip("define GUTTER_TEST_TRANSLATION=1 para probar OCR + traducción")
	}
	imagePath := os.Getenv("GUTTER_TEST_OCR_IMAGE")
	if imagePath == "" {
		t.Fatal("define GUTTER_TEST_OCR_IMAGE")
	}

	file, err := os.Open(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(file)
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}

	cfg := ocr.TranslationConfig{
		Source:   ocr.LanguageByCode("en", "en"),
		Target:   ocr.LanguageByCode("es", "es"),
		Provider: "mymemory",
	}
	result, err := ocr.BuildPageTranslation(context.Background(), img, cfg)
	if err != nil {
		t.Fatalf("ocr.BuildPageTranslation: %v", err)
	}
	if len(result.Bubbles) == 0 {
		t.Fatal("no se detectaron bocadillos")
	}
	for _, bubble := range result.Bubbles {
		t.Logf("%q -> %q en %v", bubble.Source, bubble.Text, bubble.Box)
	}

	out := filepath.Join(os.TempDir(), "gutter-translation-overlay.png")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, result.Overlay); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	t.Logf("overlay guardado en %s", out)
}
