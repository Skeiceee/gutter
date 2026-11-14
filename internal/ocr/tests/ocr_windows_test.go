//go:build windows

package tests

import (
	"context"
	"os"
	"strings"
	"testing"

	"gutter/internal/ocr"
)

// TestWindowsOCR es una prueba manual: solo se ejecuta si se indica una imagen
// con GUTTER_TEST_OCR_IMAGE. El OCR del sistema depende de los idiomas
// instalados, así que no debe formar parte del CI habitual.
func TestWindowsOCR(t *testing.T) {
	imagePath := os.Getenv("GUTTER_TEST_OCR_IMAGE")
	if imagePath == "" {
		t.Skip("define GUTTER_TEST_OCR_IMAGE para probar el OCR de Windows")
	}

	lines, err := ocr.PlatformOCR(context.Background(), imagePath, ocr.LanguageOption{Windows: "en-US"})
	if err != nil {
		t.Fatalf("PlatformOCR: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("el OCR no devolvió líneas")
	}
	all := ""
	for _, line := range lines {
		all += " " + strings.ToUpper(line.Text)
	}
	if !strings.Contains(all, "HELLO") {
		t.Fatalf("no se reconoció HELLO: %q", all)
	}
}
