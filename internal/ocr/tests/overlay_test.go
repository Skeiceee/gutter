package tests

import (
	"image"
	"testing"

	"gutter/internal/ocr"
)

func TestRenderTranslationOverlay(t *testing.T) {
	bounds := image.Rect(0, 0, 300, 200)
	bubbles := []ocr.TranslatedBubble{{
		Box:    image.Rect(20, 20, 280, 120),
		Source: "HELLO, WORLD.",
		Text:   "HOLA, MUNDO.",
	}}

	overlay, err := ocr.RenderTranslationOverlay(bounds, bubbles)
	if err != nil {
		t.Fatalf("RenderTranslationOverlay: %v", err)
	}
	if overlay.Bounds() != image.Rect(0, 0, 300, 200) {
		t.Fatalf("bounds = %v", overlay.Bounds())
	}

	// Fuera del bocadillo la capa debe ser transparente.
	if _, _, _, alpha := overlay.At(5, 5).RGBA(); alpha != 0 {
		t.Fatalf("el exterior del bocadillo debe ser transparente, alpha=%d", alpha)
	}

	// Dentro hay fondo blanco semitransparente y texto oscuro.
	_, _, _, alpha := overlay.At(25, 25).RGBA()
	if alpha == 0 {
		t.Fatal("el fondo del bocadillo no se dibujó")
	}
	darkPixels := 0
	for y := 20; y < 120; y++ {
		for x := 20; x < 280; x++ {
			r, g, b, a := overlay.At(x, y).RGBA()
			if a > 0 && r < 0x4000 && g < 0x4000 && b < 0x4000 {
				darkPixels++
			}
		}
	}
	if darkPixels == 0 {
		t.Fatal("no se dibujó texto oscuro sobre el bocadillo")
	}
}

func TestContainsCJK(t *testing.T) {
	if !ocr.ContainsCJK("こんにちは世界") {
		t.Fatal("debería detectar japonés/chino")
	}
	if !ocr.ContainsCJK("안녕하세요") {
		t.Fatal("debería detectar coreano")
	}
	if ocr.ContainsCJK("Hola, mundo") {
		t.Fatal("no debería detectar CJK en texto latino")
	}
}
