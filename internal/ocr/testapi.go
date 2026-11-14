package ocr

// testapi.go expone los identificadores internos que necesita el paquete de
// pruebas externalizado en internal/ocr/tests.

import (
	"context"
	"image"
)

// OCRLine da nombre exportado a la línea reconocida por el motor de OCR.
type OCRLine = ocrLine

// OCRBubble da nombre exportado al bocadillo agrupado.
type OCRBubble = ocrBubble

// LanguageOption da nombre exportado a la configuración de idioma.
type LanguageOption = languageOption

func GroupOCRLines(lines []OCRLine, bounds image.Rectangle) []OCRBubble {
	return groupOCRLines(lines, bounds)
}

func ParseTesseractTSV(tsv string) []OCRLine { return parseTesseractTSV(tsv) }

func RectFromFloats(x, y, w, h float64) image.Rectangle { return rectFromFloats(x, y, w, h) }

func WithEnv(base []string, values ...string) []string { return withEnv(base, values...) }

func PlatformOCR(ctx context.Context, imagePath string, source LanguageOption) ([]OCRLine, error) {
	return platformOCR(ctx, imagePath, source)
}

func ContainsCJK(text string) bool { return containsCJK(text) }

func SplitForTranslation(text string, maxBytes int) []string {
	return splitForTranslation(text, maxBytes)
}

func TranslateWithLibreTranslate(ctx context.Context, cfg TranslationConfig, text string) (string, error) {
	return translateWithLibreTranslate(ctx, cfg, text)
}

func TranslateWithDeepL(ctx context.Context, cfg TranslationConfig, text string) (string, error) {
	return translateWithDeepL(ctx, cfg, text)
}

func TranslateBubbleText(ctx context.Context, cfg TranslationConfig, text string) (string, error) {
	return translateBubbleText(ctx, cfg, text)
}

func DeepLCode(code string, source bool) string { return deepLCode(code, source) }
