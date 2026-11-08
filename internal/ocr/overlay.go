// overlay.go compone la capa que sustituye los bocadillos originales por el
// texto traducido. Se dibuja en un RGBA del mismo tamaño que la página y Fyne
// lo escala junto con ella, así que el zoom y el scroll no requieren cálculos
// adicionales.
package ocr

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"os"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	overlayFontOnce sync.Once
	overlayFont     *opentype.Font
	cjkFontOnce     sync.Once
	cjkFont         *opentype.Font
)

func RenderTranslationOverlay(bounds image.Rectangle, bubbles []TranslatedBubble) (*image.RGBA, error) {
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, errors.New("la página no tiene un tamaño válido para la traducción")
	}
	latinTypeface := translationTypeface()
	if latinTypeface == nil {
		return nil, errors.New("no se pudo cargar la tipografía para la traducción")
	}
	var cjkTypeface *opentype.Font

	overlay := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	drawn := 0
	for _, bubble := range bubbles {
		box := bubble.Box.Sub(bounds.Min).Intersect(overlay.Bounds())
		if box.Dx() <= 0 || box.Dy() <= 0 {
			continue
		}
		text := strings.TrimSpace(bubble.Text)
		if text == "" {
			text = bubble.Source
		}
		if strings.TrimSpace(text) == "" {
			continue
		}

		typeface := latinTypeface
		if containsCJK(text) {
			if cjkTypeface == nil {
				cjkTypeface = systemCJKTypeface()
			}
			if cjkTypeface == nil {
				// Sin fuente CJK es preferible dejar el bocadillo original
				// antes que dibujar caracteres vacíos.
				continue
			}
			typeface = cjkTypeface
		}

		tile := image.NewRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
		// Fondo opaco: debe tapar el texto original para que no se vea por
		// debajo de la traducción.
		draw.Draw(tile, tile.Bounds(), image.NewUniform(color.NRGBA{R: 0xFB, G: 0xFB, B: 0xF8, A: 0xFF}), image.Point{}, draw.Src)
		drawBorder(tile, color.NRGBA{R: 0x00, G: 0x7A, B: 0xCC, A: 0xD8}, 2)
		drawBubbleText(tile, typeface, text)
		draw.Draw(overlay, box, tile, image.Point{}, draw.Over)
		drawn++
	}
	if drawn == 0 {
		return nil, errors.New("no se pudo dibujar ninguna traducción (¿falta una fuente para ese idioma?)")
	}
	return overlay, nil
}

func translationTypeface() *opentype.Font {
	overlayFontOnce.Do(func() {
		parsed, err := opentype.Parse(goregular.TTF)
		if err == nil {
			overlayFont = parsed
		}
	})
	return overlayFont
}

func containsCJK(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) ||
			unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) ||
			unicode.Is(unicode.Hangul, r) {
			return true
		}
	}
	return false
}

// systemCJKTypeface intenta usar una fuente del sistema para destinos u OCR en
// chino, japonés o coreano. Si no hay ninguna, se cae a la fuente Go (tofu).
func systemCJKTypeface() *opentype.Font {
	cjkFontOnce.Do(func() {
		candidates := []string{
			`C:\Windows\Fonts\msyh.ttc`,
			`C:\Windows\Fonts\meiryo.ttc`,
			`C:\Windows\Fonts\malgun.ttf`,
			`C:\Windows\Fonts\simhei.ttf`,
			`/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc`,
			`/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc`,
			`/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc`,
			`/System/Library/Fonts/PingFang.ttc`,
			`/System/Library/Fonts/STHeiti Light.ttc`,
		}
		for _, path := range candidates {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if collection, err := opentype.ParseCollection(data); err == nil && collection.NumFonts() > 0 {
				if parsed, err := collection.Font(0); err == nil {
					cjkFont = parsed
					return
				}
			}
			if parsed, err := opentype.Parse(data); err == nil {
				cjkFont = parsed
				return
			}
		}
	})
	return cjkFont
}

func drawBubbleText(dst *image.RGBA, typeface *opentype.Font, text string) {
	innerWidth := float64(dst.Bounds().Dx() - 14)
	innerHeight := float64(dst.Bounds().Dy() - 12)
	if innerWidth < 8 || innerHeight < 8 {
		return
	}

	startSize := int(float64(dst.Bounds().Dy()) * 0.55)
	if startSize > 54 {
		startSize = 54
	}
	if startSize < 10 {
		startSize = 10
	}

	for size := startSize; size >= 7; size-- {
		face, err := opentype.NewFace(typeface, &opentype.FaceOptions{
			Size:    float64(size),
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			return
		}
		lines := wrapOverlayText(face, text, innerWidth)
		lineHeight := float64(face.Metrics().Height.Ceil()) * 1.08
		totalHeight := lineHeight * float64(len(lines))
		if totalHeight > innerHeight && size > 7 {
			_ = face.Close()
			continue
		}

		ascent := face.Metrics().Ascent.Ceil()
		y := (dst.Bounds().Dy()-int(totalHeight))/2 + ascent
		drawer := &font.Drawer{
			Dst:  dst,
			Src:  image.NewUniform(color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xFF}),
			Face: face,
		}
		for _, line := range lines {
			width := font.MeasureString(face, line).Ceil()
			drawer.Dot = fixed.P((dst.Bounds().Dx()-width)/2, y)
			drawer.DrawString(line)
			y += int(lineHeight)
		}
		_ = face.Close()
		return
	}
}

// wrapOverlayText reparte el texto en líneas y parte palabras que no caben.
func wrapOverlayText(face font.Face, text string, maxWidth float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if font.MeasureString(face, candidate).Ceil() <= int(maxWidth) {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		if font.MeasureString(face, word).Ceil() <= int(maxWidth) {
			current = word
			continue
		}
		parts := splitWordToWidth(face, word, maxWidth)
		if len(parts) > 0 {
			lines = append(lines, parts[:len(parts)-1]...)
			current = parts[len(parts)-1]
		} else {
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitWordToWidth(face font.Face, word string, maxWidth float64) []string {
	runes := []rune(word)
	if len(runes) == 0 {
		return nil
	}
	var parts []string
	var current []rune
	for _, r := range runes {
		candidate := string(append(current, r))
		if len(current) > 0 && font.MeasureString(face, candidate).Ceil() > int(maxWidth) {
			parts = append(parts, string(current))
			current = []rune{r}
			continue
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

func drawBorder(dst *image.RGBA, borderColor color.Color, thickness int) {
	if thickness <= 0 {
		return
	}
	uniform := image.NewUniform(borderColor)
	b := dst.Bounds()
	draw.Draw(dst, image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+thickness), uniform, image.Point{}, draw.Over)
	draw.Draw(dst, image.Rect(b.Min.X, b.Max.Y-thickness, b.Max.X, b.Max.Y), uniform, image.Point{}, draw.Over)
	draw.Draw(dst, image.Rect(b.Min.X, b.Min.Y, b.Min.X+thickness, b.Max.Y), uniform, image.Point{}, draw.Over)
	draw.Draw(dst, image.Rect(b.Max.X-thickness, b.Min.Y, b.Max.X, b.Max.Y), uniform, image.Point{}, draw.Over)
}
