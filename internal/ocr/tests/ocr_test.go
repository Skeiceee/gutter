package tests

import (
	"image"
	"strings"
	"testing"

	"gutter/internal/ocr"
)

func TestGroupOCRLinesMergesMultilineBubble(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 1000)
	lines := []ocr.OCRLine{
		{Text: "HELLO,", Rect: image.Rect(100, 100, 300, 130)},
		{Text: "I AM A BUBBLE.", Rect: image.Rect(120, 140, 320, 170)},
		{Text: "A SEPARATE BUBBLE.", Rect: image.Rect(500, 500, 700, 530)},
	}

	bubbles := ocr.GroupOCRLines(lines, bounds)
	if len(bubbles) != 2 {
		t.Fatalf("se esperaban 2 bocadillos, hay %d: %+v", len(bubbles), bubbles)
	}
	if !strings.Contains(bubbles[0].Text, "HELLO") || !strings.Contains(bubbles[0].Text, "BUBBLE") {
		t.Fatalf("el primer bocadillo no agrupó sus líneas: %q", bubbles[0].Text)
	}
	if bubbles[0].Rect.Dx() <= 220 || bubbles[0].Rect.Dy() <= 70 {
		t.Fatalf("la caja del bocadillo no incluye el padding esperado: %v", bubbles[0].Rect)
	}
}

func TestGroupOCRLinesKeepsSideBySideBubblesSeparate(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 600)
	lines := []ocr.OCRLine{
		{Text: "LEFT BUBBLE", Rect: image.Rect(50, 100, 250, 130)},
		{Text: "RIGHT BUBBLE", Rect: image.Rect(400, 100, 620, 130)},
	}

	bubbles := ocr.GroupOCRLines(lines, bounds)
	if len(bubbles) != 2 {
		t.Fatalf("los bocadillos laterales no deben unirse: %+v", bubbles)
	}
}

func TestGroupOCRLinesFiltersEmptyText(t *testing.T) {
	bubbles := ocr.GroupOCRLines([]ocr.OCRLine{
		{Text: " ", Rect: image.Rect(10, 10, 30, 30)},
		{Text: "", Rect: image.Rect(40, 40, 60, 60)},
	}, image.Rect(0, 0, 100, 100))
	if len(bubbles) != 0 {
		t.Fatalf("no se esperaban bocadillos, hay %d", len(bubbles))
	}
}

func TestParseTesseractTSV(t *testing.T) {
	tsv := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"5\t1\t1\t1\t1\t1\t10\t20\t50\t20\t95\tHELLO\n" +
		"5\t1\t1\t1\t1\t2\t65\t20\t40\t20\t92\tWORLD\n" +
		"5\t1\t1\t1\t2\t1\t12\t45\t30\t18\t88\tSECOND\n" +
		"5\t1\t1\t1\t3\t1\t12\t75\t30\t18\t10\tLOWCONF\n"

	lines := ocr.ParseTesseractTSV(tsv)
	if len(lines) != 2 {
		t.Fatalf("se esperaban 2 líneas, hay %d: %+v", len(lines), lines)
	}
	if lines[0].Text != "HELLO WORLD" {
		t.Fatalf("primera línea = %q", lines[0].Text)
	}
	if lines[1].Text != "SECOND" {
		t.Fatalf("segunda línea = %q", lines[1].Text)
	}
	if lines[0].Rect != image.Rect(10, 20, 105, 40) {
		t.Fatalf("caja de la primera línea = %v", lines[0].Rect)
	}
}

func TestRectFromFloats(t *testing.T) {
	got := ocr.RectFromFloats(10.4, 20.6, 30.2, 40.8)
	want := image.Rect(10, 21, 41, 61)
	if got != want {
		t.Fatalf("RectFromFloats = %v, quiero %v", got, want)
	}
	if !ocr.RectFromFloats(0, 0, 0, 0).Empty() {
		t.Fatal("una caja vacía debe quedar vacía")
	}
}

func TestWithEnvReplacesWithoutDuplicates(t *testing.T) {
	env := ocr.WithEnv(
		[]string{"PATH=C:\\bin", "GUTTER_OCR_LANG=old"},
		"GUTTER_OCR_LANG", "en-US",
		"GUTTER_OCR_IMAGE", `C:\temp\page.png`,
	)
	joined := strings.Join(env, "\n")
	if strings.Count(joined, "GUTTER_OCR_LANG=") != 1 {
		t.Fatalf("la clave no debe duplicarse: %v", env)
	}
	if !strings.Contains(joined, "GUTTER_OCR_LANG=en-US") {
		t.Fatalf("no se reemplazó el valor: %v", env)
	}
}
