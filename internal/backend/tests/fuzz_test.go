package tests

// fuzz_test.go cubre los parsers que reciben bytes no confiables: cabeceras de
// imagen (PNG/JPEG/GIF/WebP/AVIF/BMP/TIFF), el formato binario .gtp de la caché
// en disco y la detección/indexado de contenedores RAR/ZIP.
//
// El objetivo no es validar casos concretos, sino garantizar que ninguna
// entrada corrupta provoca un pánico, una reserva desmedida o un bucle. Los
// seeds se ejecutan en cada `go test`; el fuzzing real se lanza con
// `go test -fuzz=Fuzz... ./internal/backend/tests`.

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"gutter/internal/backend"
)

// samplePNG construye un PNG válido mínimo sin depender de *testing.T.
func samplePNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0xFF, A: 0xFF})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func FuzzDecodeImage(f *testing.F) {
	f.Add(samplePNG())
	f.Add([]byte("no es una imagen"))
	f.Add([]byte{})
	f.Add([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Las imágenes reales de un cómic caben de sobra en 1 MiB. Limitar la
		// entrada evita que el fuzzer explore reservas gigantes por accidente.
		if len(data) > 1<<20 {
			t.Skip("entrada demasiado grande")
		}
		img, err := backend.DecodeImageForTest(data, "page.png")
		if err != nil {
			return
		}
		if img == nil {
			t.Fatal("DecodeImage devolvió nil sin error")
		}
		if img.Bounds().Empty() {
			t.Fatal("imagen decodificada sin dimensiones")
		}
	})
}

func FuzzDecodeDiskPage(f *testing.F) {
	valid := make([]byte, 16+4)
	copy(valid[0:4], "GTP1")
	binary.LittleEndian.PutUint32(valid[4:8], backend.DiskPageVersionForTest)
	binary.LittleEndian.PutUint32(valid[8:12], 1)
	binary.LittleEndian.PutUint32(valid[12:16], 1)
	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("GTP1"))
	f.Add([]byte("GTP1\x02\x00\x00\x00\xff\xff\xff\xff\xff\xff\xff\xff"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip("entrada demasiado grande")
		}
		img, err := backend.DecodeDiskPageForTest(data)
		if err != nil {
			return
		}
		if img == nil {
			t.Fatal("DecodeDiskPage devolvió nil sin error")
		}
		if got := img.Bounds().Dx() * img.Bounds().Dy() * 4; got != len(data)-16 {
			t.Fatalf("tamaño declarado %d, bytes de píxeles %d", got, len(data)-16)
		}
	})
}

func FuzzOpenComicHeaders(f *testing.F) {
	f.Add([]byte("PK\x03\x04"), ".cbz")
	f.Add([]byte("PK\x05\x06"), ".cbz")
	f.Add([]byte("Rar!\x1a\x07\x01\x00"), ".cbr")
	f.Add([]byte("Rar!\x1a\x07\x00"), ".rar")
	f.Add([]byte("no es un contenedor"), ".cbz")
	f.Add([]byte{}, ".cbr")

	// Un único directorio para todas las iteraciones: crear y borrar un
	// TempDir por entrada dominaba el tiempo del fuzzer.
	dir, err := os.MkdirTemp("", "gutter-fuzz-*")
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() { _ = os.RemoveAll(dir) })

	f.Fuzz(func(t *testing.T, data []byte, ext string) {
		if len(data) > 1<<16 {
			t.Skip("entrada demasiado grande")
		}
		if ext != ".cbz" && ext != ".cbr" {
			ext = ".cbz"
		}
		comicPath := filepath.Join(dir, "fuzz"+ext)
		if err := os.WriteFile(comicPath, data, 0o644); err != nil {
			t.Fatal(err)
		}

		comic, err := backend.OpenComic(comicPath)
		if err != nil {
			return
		}
		if comic == nil {
			t.Fatal("OpenComic devolvió nil sin error")
		}
		// Las páginas deben ser nombres utilizables; no se descomprime nada.
		for _, page := range comic.Pages {
			if page == "" {
				t.Fatal("OpenComic indexó una página con nombre vacío")
			}
		}
		_ = comic.Problems()
		if err := comic.Close(); err != nil {
			t.Fatalf("Close falló tras indexar: %v", err)
		}
	})
}
