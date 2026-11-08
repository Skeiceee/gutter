// ocr.go contiene el contrato común del OCR y la agrupación de líneas en
// bocadillos. El motor concreto depende de la plataforma:
//
//   - Windows: OCR integrado (Windows.Media.Ocr) vía PowerShell, sin instalar
//     nada adicional.
//   - Resto: Tesseract CLI si está en el PATH (o en TESSERACT_PATH).
//
// El OCR se ejecuta siempre bajo demanda: nunca al abrir una página.
package ocr

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"sort"
	"strings"
)

// ocrLine es una línea de texto reconocida y su caja en píxeles de la imagen.
type ocrLine struct {
	Text string
	Rect image.Rectangle
}

// ocrBubble agrupa varias líneas que pertenecen al mismo bocadillo.
type ocrBubble struct {
	Text string
	Rect image.Rectangle
}

// recognizeText elige el mejor motor disponible. En Windows intenta primero
// el OCR del sistema; si falla (por ejemplo, idioma no instalado), cae a
// Tesseract cuando está disponible.
func recognizeText(ctx context.Context, img image.Image, source languageOption) ([]ocrLine, error) {
	imagePath, err := writeOCRTempPNG(img)
	if err != nil {
		return nil, err
	}
	defer os.Remove(imagePath)

	platformLines, platformErr := platformOCR(ctx, imagePath, source)
	if platformErr == nil {
		return platformLines, nil
	}

	tesseractLines, tesseractErr := tesseractOCR(ctx, imagePath, source.Tesseract)
	if tesseractErr == nil {
		return tesseractLines, nil
	}
	return nil, fmt.Errorf("%v; %v", platformErr, tesseractErr)
}

// rectFromFloats redondea la caja que devuelve Windows OCR a píxeles.
func rectFromFloats(x, y, w, h float64) image.Rectangle {
	if w <= 0 || h <= 0 {
		return image.Rectangle{}
	}
	return image.Rect(
		int(math.Round(x)),
		int(math.Round(y)),
		int(math.Round(x+w)),
		int(math.Round(y+h)),
	)
}

// withEnv reemplaza las claves indicadas sin dejar duplicados en el entorno.
func withEnv(base []string, values ...string) []string {
	replace := make(map[string]string, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		replace[strings.ToUpper(values[i])] = values[i+1]
	}
	out := make([]string, 0, len(base)+len(replace))
	for _, item := range base {
		name := item
		if idx := strings.IndexByte(item, '='); idx >= 0 {
			name = item[:idx]
		}
		if _, ok := replace[strings.ToUpper(name)]; ok {
			continue
		}
		out = append(out, item)
	}
	for key, value := range replace {
		out = append(out, key+"="+value)
	}
	return out
}

// writeOCRTempPNG normaliza la imagen a PNG en un archivo temporal. Los dos
// motores aceptan PNG y así evitamos duplicar codificadores.
func writeOCRTempPNG(img image.Image) (string, error) {
	if img == nil {
		return "", errors.New("no hay imagen para reconocer")
	}
	f, err := os.CreateTemp("", "gutter-ocr-*.png")
	if err != nil {
		return "", fmt.Errorf("no se pudo crear el archivo temporal de OCR: %w", err)
	}
	name := f.Name()
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", fmt.Errorf("no se pudo codificar la página para OCR: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", fmt.Errorf("no se pudo cerrar el archivo temporal de OCR: %w", err)
	}
	return name, nil
}

// groupOCRLines convierte líneas sueltas en bocadillos. Dos líneas se unen si
// están verticalmente cerca y comparten espacio horizontal. Es una heurística
// suficiente para bocadillos de cómic y evita depender del layout del motor.
func groupOCRLines(lines []ocrLine, bounds image.Rectangle) []ocrBubble {
	filtered := make([]ocrLine, 0, len(lines))
	for _, line := range lines {
		line.Text = strings.TrimSpace(line.Text)
		if line.Text == "" || line.Rect.Dx() <= 0 || line.Rect.Dy() <= 0 {
			continue
		}
		line.Rect = clampRect(line.Rect, bounds)
		if line.Rect.Dx() <= 0 || line.Rect.Dy() <= 0 {
			continue
		}
		filtered = append(filtered, line)
	}
	if len(filtered) == 0 {
		return nil
	}

	parent := make([]int, len(filtered))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	for i := 0; i < len(filtered); i++ {
		for j := i + 1; j < len(filtered); j++ {
			if linesBelongTogether(filtered[i], filtered[j]) {
				union(i, j)
			}
		}
	}

	groups := make(map[int][]ocrLine)
	for i, line := range filtered {
		root := find(i)
		groups[root] = append(groups[root], line)
	}

	bubbles := make([]ocrBubble, 0, len(groups))
	for _, group := range groups {
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Rect.Min.Y == group[j].Rect.Min.Y {
				return group[i].Rect.Min.X < group[j].Rect.Min.X
			}
			return group[i].Rect.Min.Y < group[j].Rect.Min.Y
		})

		texts := make([]string, 0, len(group))
		rect := group[0].Rect
		for _, line := range group {
			texts = append(texts, line.Text)
			rect = unionRect(rect, line.Rect)
		}
		text := strings.TrimSpace(strings.Join(texts, " "))
		if len([]rune(text)) < 2 {
			continue
		}
		bubbles = append(bubbles, ocrBubble{
			Text: text,
			Rect: padRect(rect, 7, bounds),
		})
	}

	sort.SliceStable(bubbles, func(i, j int) bool {
		if bubbles[i].Rect.Min.Y == bubbles[j].Rect.Min.Y {
			return bubbles[i].Rect.Min.X < bubbles[j].Rect.Min.X
		}
		return bubbles[i].Rect.Min.Y < bubbles[j].Rect.Min.Y
	})
	return bubbles
}

// linesBelongTogether decide si dos líneas están en el mismo bocadillo.
// Se permite un pequeño hueco vertical y se exige solapamiento horizontal.
func linesBelongTogether(a, b ocrLine) bool {
	avgHeight := float64(a.Rect.Dy()+b.Rect.Dy()) / 2
	if avgHeight <= 0 {
		return false
	}

	verticalGap := maxInt(0, maxInt(a.Rect.Min.Y, b.Rect.Min.Y)-minInt(a.Rect.Max.Y, b.Rect.Max.Y))
	if float64(verticalGap) > avgHeight*0.75 {
		return false
	}

	overlap := minInt(a.Rect.Max.X, b.Rect.Max.X) - maxInt(a.Rect.Min.X, b.Rect.Min.X)
	if overlap > 0 {
		minWidth := minInt(a.Rect.Dx(), b.Rect.Dx())
		return float64(overlap) >= float64(minWidth)*0.25
	}

	// Líneas centradas: pueden no solaparse si una es mucho más corta, pero
	// deben estar casi pegadas horizontalmente.
	horizontalGap := -overlap
	return float64(horizontalGap) <= avgHeight*0.35
}

func unionRect(a, b image.Rectangle) image.Rectangle {
	return image.Rect(
		minInt(a.Min.X, b.Min.X),
		minInt(a.Min.Y, b.Min.Y),
		maxInt(a.Max.X, b.Max.X),
		maxInt(a.Max.Y, b.Max.Y),
	)
}

func padRect(r image.Rectangle, padding int, bounds image.Rectangle) image.Rectangle {
	return clampRect(
		image.Rect(r.Min.X-padding, r.Min.Y-padding, r.Max.X+padding, r.Max.Y+padding),
		bounds,
	)
}

func clampRect(r image.Rectangle, bounds image.Rectangle) image.Rectangle {
	return r.Intersect(bounds)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
