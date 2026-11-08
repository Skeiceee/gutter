package ocr

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// tesseractOCR es el respaldo multiplataforma. Usa la salida TSV para obtener
// palabras con coordenadas y luego las agrupa por línea.
func tesseractOCR(ctx context.Context, imagePath, language string) ([]ocrLine, error) {
	binary := findTesseract()
	if binary == "" {
		return nil, errors.New(
			"Tesseract no está instalado. Instálalo y añade 'tesseract' al PATH, " +
				"o define TESSERACT_PATH",
		)
	}
	if language == "" {
		language = "eng"
	}

	cmd := exec.CommandContext(
		ctx,
		binary,
		imagePath,
		"stdout",
		"-l", language,
		"--psm", "3",
		"tsv",
	)
	hideConsole(cmd)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("Tesseract falló: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("no se pudo ejecutar Tesseract: %w", err)
	}
	return parseTesseractTSV(string(output)), nil
}

func findTesseract() string {
	if path := strings.TrimSpace(os.Getenv("TESSERACT_PATH")); path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	if path, err := exec.LookPath("tesseract"); err == nil {
		return path
	}

	candidates := []string{
		`C:\Program Files\Tesseract-OCR\tesseract.exe`,
		`C:\Program Files (x86)\Tesseract-OCR\tesseract.exe`,
		`/usr/bin/tesseract`,
		`/usr/local/bin/tesseract`,
		`/opt/homebrew/bin/tesseract`,
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.FromSlash(candidate)); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func parseTesseractTSV(tsv string) []ocrLine {
	type key struct {
		block int
		par   int
		line  int
	}
	type group struct {
		words []string
		rect  image.Rectangle
	}

	groups := make(map[key]*group)
	scanner := bufio.NewScanner(strings.NewReader(tsv))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 12 || fields[0] != "5" {
			continue
		}
		text := strings.TrimSpace(fields[11])
		if text == "" {
			continue
		}
		conf, err := strconv.Atoi(fields[10])
		if err != nil || conf < 30 {
			continue
		}
		left, _ := strconv.Atoi(fields[6])
		top, _ := strconv.Atoi(fields[7])
		width, _ := strconv.Atoi(fields[8])
		height, _ := strconv.Atoi(fields[9])
		if width <= 0 || height <= 0 {
			continue
		}
		block, _ := strconv.Atoi(fields[2])
		par, _ := strconv.Atoi(fields[3])
		line, _ := strconv.Atoi(fields[4])
		k := key{block: block, par: par, line: line}
		wordRect := image.Rect(left, top, left+width, top+height)
		g := groups[k]
		if g == nil {
			groups[k] = &group{words: []string{text}, rect: wordRect}
			continue
		}
		g.words = append(g.words, text)
		g.rect = unionRect(g.rect, wordRect)
	}

	lines := make([]ocrLine, 0, len(groups))
	for _, g := range groups {
		if len(g.words) == 0 {
			continue
		}
		lines = append(lines, ocrLine{
			Text: strings.Join(g.words, " "),
			Rect: g.rect,
		})
	}
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].Rect.Min.Y == lines[j].Rect.Min.Y {
			return lines[i].Rect.Min.X < lines[j].Rect.Min.X
		}
		return lines[i].Rect.Min.Y < lines[j].Rect.Min.Y
	})
	return lines
}
