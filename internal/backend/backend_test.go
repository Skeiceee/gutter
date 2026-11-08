package backend

import (
	"archive/zip"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNaturalLess(t *testing.T) {
	pages := []string{
		"page10.jpg",
		"page2.jpg",
		"Page1.jpg",
		"page02.jpg",
	}
	want := []string{"Page1.jpg", "page02.jpg", "page2.jpg", "page10.jpg"}

	sort.SliceStable(pages, func(i, j int) bool { return naturalLess(pages[i], pages[j]) })
	for i := range want {
		if pages[i] != want[i] {
			t.Fatalf("orden[%d] = %q, quiero %q (orden completo: %v)", i, pages[i], want[i], pages)
		}
	}
}

func TestNaturalCompareExamples(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"page1.jpg", "page2.jpg", -1},
		{"page2.jpg", "page10.jpg", -1},
		{"page10.jpg", "page2.jpg", 1},
		{"001.jpg", "1.jpg", 0},
		{"a.jpg", "b.jpg", -1},
	}
	for _, tc := range cases {
		if got := naturalCompare(tc.a, tc.b); got != tc.want {
			t.Errorf("naturalCompare(%q, %q) = %d, quiero %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCropHalfCompactAndCorrect(t *testing.T) {
	// 10x4: mitad izquierda roja, mitad derecha azul.
	img := image.NewRGBA(image.Rect(0, 0, 10, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 10; x++ {
			if x < 5 {
				img.Set(x, y, color.RGBA{R: 0xFF, A: 0xFF})
			} else {
				img.Set(x, y, color.RGBA{B: 0xFF, A: 0xFF})
			}
		}
	}

	left := CropHalf(img, false)
	rb, ok := left.(*image.RGBA)
	if !ok {
		t.Fatalf("la mitad izquierda debe ser *image.RGBA, es %T", left)
	}
	if got := left.Bounds(); got != image.Rect(0, 0, 5, 4) {
		t.Errorf("bounds izquierda = %v", got)
	}
	if rb.Stride != 5*4 {
		t.Errorf("la mitad debe estar compactada: stride=%d", rb.Stride)
	}
	if r, g, b, _ := left.At(0, 0).RGBA(); r != 0xFFFF || g != 0 || b != 0 {
		t.Errorf("píxel izquierdo inesperado: %v", left.At(0, 0))
	}

	right := CropHalf(img, true)
	if got := right.Bounds(); got != image.Rect(0, 0, 5, 4) {
		t.Errorf("bounds derecha = %v", got)
	}
	if r, g, b, _ := right.At(4, 3).RGBA(); r != 0 || g != 0 || b != 0xFFFF {
		t.Errorf("píxel derecho inesperado: %v", right.At(4, 3))
	}
}

func TestToRGBAFromPaletted(t *testing.T) {
	pal := color.Palette{
		color.RGBA{A: 0xFF},
		color.RGBA{R: 0xFF, A: 0xFF},
	}
	src := image.NewPaletted(image.Rect(0, 0, 2, 1), pal)
	src.SetColorIndex(0, 0, 1)
	src.SetColorIndex(1, 0, 0)

	rgba := toRGBA(src)
	if _, ok := image.Image(rgba).(*image.RGBA); !ok {
		t.Fatalf("toRGBA devolvió %T", rgba)
	}
	if r, _, _, _ := rgba.At(0, 0).RGBA(); r != 0xFFFF {
		t.Errorf("conversión de paleta incorrecta: %v", rgba.At(0, 0))
	}
}

func TestIsComicImageAndHidden(t *testing.T) {
	valid := []string{"001.JPG", "page.webp", "cover.avif", "scan.tiff"}
	for _, name := range valid {
		if !isComicImage(name) {
			t.Errorf("%q debería ser una imagen válida", name)
		}
	}
	invalid := []string{"info.txt", "ComicInfo.xml", "Thumbs.db", "page.psd"}
	for _, name := range invalid {
		if isComicImage(name) {
			t.Errorf("%q no debería aceptarse", name)
		}
	}
	if !isHiddenComicPath("__MACOSX/._page.jpg") {
		t.Error("__MACOSX debe ignorarse")
	}
	if !isHiddenComicPath(".hidden/page.jpg") {
		t.Error("las carpetas ocultas deben ignorarse")
	}
	if isHiddenComicPath("Chapter 01/page.jpg") {
		t.Error("una ruta normal no es oculta")
	}
}

func TestOpenZIPComic(t *testing.T) {
	// Muchos .cbr descargados de internet son en realidad ZIP. El archivo de
	// prueba usa la extensión .cbr a propósito.
	comicPath := filepath.Join(t.TempDir(), "falso.cbr")
	f, err := os.Create(comicPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	for _, name := range []string{"10.png", "2.png"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 8, 6))
		for y := 0; y < 6; y++ {
			for x := 0; x < 8; x++ {
				img.Set(x, y, color.RGBA{R: 0x20, G: 0x80, B: 0xC0, A: 0xFF})
			}
		}
		if err := png.Encode(w, img); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := zw.Create("ComicInfo.xml"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	comic, err := OpenComic(comicPath)
	if err != nil {
		t.Fatal(err)
	}
	defer comic.Close()

	want := []string{"2.png", "10.png"}
	if len(comic.Pages) != len(want) {
		t.Fatalf("páginas = %v, quiero %v", comic.Pages, want)
	}
	for i := range want {
		if comic.Pages[i] != want[i] {
			t.Fatalf("páginas = %v, quiero %v", comic.Pages, want)
		}
	}

	page, err := comic.Page("2.png")
	if err != nil {
		t.Fatal(err)
	}
	if got := page.Bounds(); got != image.Rect(0, 0, 8, 6) {
		t.Errorf("bounds = %v", got)
	}
}
