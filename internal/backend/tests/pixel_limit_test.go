package tests

// pixel_limit_test.go comprueba que una entrada que declara dimensiones
// enormes se rechaza antes de que el decodificador reserve memoria para todos
// sus píxeles. maxPageBytes acota los bytes comprimidos, no las dimensiones:
// un PNG de pocos KB puede declarar 100 000 × 100 000 píxeles (~40 GB en RGBA).

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gutter/internal/backend"
)

func TestPageRejectsPixelBomb(t *testing.T) {
	comicPath := writeComicWithEntry(t, "bomb.png", oversizedPNG(100_000, 100_000))

	comic, err := backend.OpenComic(comicPath)
	if err != nil {
		t.Fatal(err)
	}
	defer comic.Close()

	if _, err := comic.Page("bomb.png"); err == nil || !strings.Contains(err.Error(), "límite") {
		t.Fatalf("Page debe rechazar la bomba de píxeles, err = %v", err)
	}
	if _, err := comic.Thumbnail("bomb.png", 160); err == nil || !strings.Contains(err.Error(), "límite") {
		t.Fatalf("Thumbnail debe rechazar la bomba de píxeles, err = %v", err)
	}
}

// writeComicWithEntry crea un .cbz de un solo archivo con los bytes indicados.
func writeComicWithEntry(t *testing.T, name string, data []byte) string {
	t.Helper()

	comicPath := filepath.Join(t.TempDir(), "comic.cbz")
	f, err := os.Create(comicPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return comicPath
}

// oversizedPNG construye un PNG con cabecera válida y dimensiones arbitrarias.
// No incluye datos de imagen: basta con que image.DecodeConfig lo acepte.
func oversizedPNG(width, height uint32) []byte {
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], width)
	binary.BigEndian.PutUint32(ihdr[4:8], height)
	ihdr[8] = 8 // profundidad de bits
	ihdr[9] = 6 // color RGBA
	writePNGChunk(&buf, "IHDR", ihdr)
	writePNGChunk(&buf, "IDAT", nil)
	return buf.Bytes()
}

func writePNGChunk(w io.Writer, kind string, data []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(data)))
	_, _ = w.Write(length[:])
	_, _ = io.WriteString(w, kind)
	_, _ = w.Write(data)

	crc := crc32.NewIEEE()
	_, _ = crc.Write([]byte(kind))
	_, _ = crc.Write(data)

	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], crc.Sum32())
	_, _ = w.Write(sum[:])
}
