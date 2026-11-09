package backend

import (
	"archive/zip"
	"context"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func writeTestZIP(t *testing.T, pages map[string][]byte) string {
	t.Helper()
	comicPath := filepath.Join(t.TempDir(), "test.cbz")
	f, err := os.Create(comicPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, data := range pages {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return comicPath
}

func testPNG(t *testing.T, width, height int, rnd *rand.Rand) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if rnd != nil {
				img.Set(x, y, color.RGBA{
					R: uint8(rnd.Intn(256)),
					G: uint8(rnd.Intn(256)),
					B: uint8(rnd.Intn(256)),
					A: 0xFF,
				})
				continue
			}
			img.Set(x, y, color.RGBA{R: 0x20, G: 0x80, B: 0xC0, A: 0xFF})
		}
	}
	var out []byte
	writer := &sliceWriter{data: &out}
	if err := png.Encode(writer, img); err != nil {
		t.Fatal(err)
	}
	return out
}

type sliceWriter struct {
	data *[]byte
}

func (w *sliceWriter) Write(p []byte) (int, error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}

func TestDiskCachePageRoundTrip(t *testing.T) {
	cache := NewDiskCache(t.TempDir(), 8<<20)
	defer cache.Clear()

	img := image.NewRGBA(image.Rect(0, 0, 6, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 6; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 30), B: 0x80, A: 0xFF})
		}
	}
	cache.SavePage("comic-1", "page.png", img)

	got, ok := cache.LoadPage("comic-1", "page.png")
	if !ok {
		t.Fatal("la página no se recuperó de la caché")
	}
	if got.Bounds() != img.Bounds() {
		t.Fatalf("bounds = %v, quiero %v", got.Bounds(), img.Bounds())
	}
	if got.At(3, 2) != img.At(3, 2) {
		t.Fatalf("píxel = %v, quiero %v", got.At(3, 2), img.At(3, 2))
	}

	stats := cache.Stats()
	if stats.Pages != 1 || stats.PageBytes <= 0 {
		t.Fatalf("estadísticas inesperadas: %+v", stats)
	}

	if err := cache.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.LoadPage("comic-1", "page.png"); ok {
		t.Fatal("Clear debe borrar las páginas")
	}
	if stats := cache.Stats(); stats.Pages != 0 || stats.PageBytes != 0 {
		t.Fatalf("contadores tras Clear: %+v", stats)
	}
}

func TestDiskCacheEvictsOldPages(t *testing.T) {
	const limit = 1200
	cache := NewDiskCache(t.TempDir(), limit)
	defer cache.Clear()

	rnd := rand.New(rand.NewSource(7))
	for i := 0; i < 10; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 12, 12))
		for y := 0; y < 12; y++ {
			for x := 0; x < 12; x++ {
				img.Set(x, y, color.RGBA{
					R: uint8(rnd.Intn(256)),
					G: uint8(rnd.Intn(256)),
					B: uint8(rnd.Intn(256)),
					A: 0xFF,
				})
			}
		}
		cache.SavePage("comic-evict", "page.png", img)
	}

	stats := cache.Stats()
	if stats.PageBytes > limit {
		t.Fatalf("la caché usa %d bytes, límite %d", stats.PageBytes, limit)
	}
}

func TestDiskCacheIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cache := NewDiskCache(dir, 1<<20)
	defer cache.Clear()

	file := filepath.Join(dir, "comic.cbz")
	if err := os.WriteFile(file, []byte("datos"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	identity := archiveIdentity(file, info)
	opts := OpenOptions{Sort: SortLexicographic, IncludeJunk: true}
	problems := []PageProblem{{Name: "page10.png", Detail: "prueba"}}

	cache.SaveIndex(identity, info, opts, true, []string{"page10.png", "page2.png"}, problems)
	entry, ok := cache.LoadIndex(identity, info, opts)
	if !ok {
		t.Fatal("el índice no se recuperó")
	}
	if !entry.Solid || len(entry.Pages) != 2 || entry.Pages[0] != "page10.png" {
		t.Fatalf("índice inesperado: %+v", entry)
	}
	if len(entry.Problems) != 1 || entry.Problems[0].Name != "page10.png" {
		t.Fatalf("problemas no persistidos: %+v", entry.Problems)
	}

	if _, ok := cache.LoadIndex(identity, info, OpenOptions{Sort: SortNatural}); ok {
		t.Fatal("un cambio de orden debe invalidar la entrada")
	}
	if stats := cache.Stats(); stats.Indexes != 1 {
		t.Fatalf("índices = %d, quiero 1", stats.Indexes)
	}
}

func TestZIPSortModesAndJunkFilter(t *testing.T) {
	good := testPNG(t, 4, 4, nil)
	comicPath := writeTestZIP(t, map[string][]byte{
		"page10.png":           good,
		"page2.png":            good,
		"__MACOSX/._page3.png": good,
		".hidden/page4.png":    good,
		"sub/._resource.png":   good,
		"sub/normal-page9.png": good,
		"ComicInfo.xml":        []byte("<ComicInfo/>"),
	})

	natural, err := OpenComicWithOptions(comicPath, OpenOptions{Sort: SortNatural})
	if err != nil {
		t.Fatal(err)
	}
	defer natural.Close()
	wantNatural := []string{"page2.png", "page10.png", "sub/normal-page9.png"}
	assertPages(t, natural.Pages, wantNatural)

	lexical, err := OpenComicWithOptions(comicPath, OpenOptions{Sort: SortLexicographic})
	if err != nil {
		t.Fatal(err)
	}
	defer lexical.Close()
	wantLexical := []string{"page10.png", "page2.png", "sub/normal-page9.png"}
	assertPages(t, lexical.Pages, wantLexical)

	withJunk, err := OpenComicWithOptions(comicPath, OpenOptions{Sort: SortNatural, IncludeJunk: true})
	if err != nil {
		t.Fatal(err)
	}
	defer withJunk.Close()
	if len(withJunk.Pages) != 6 {
		t.Fatalf("con IncludeJunk se esperaban 6 páginas, hay %v", withJunk.Pages)
	}
}

func assertPages(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("páginas = %v, quiero %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("páginas = %v, quiero %v", got, want)
		}
	}
}

func TestIntegrityCheckFlagsBrokenImage(t *testing.T) {
	good := testPNG(t, 4, 4, nil)
	comicPath := writeTestZIP(t, map[string][]byte{
		"1.png": good,
		"2.png": []byte("esto no es una imagen"),
	})

	comic, err := OpenComicWithOptions(comicPath, OpenOptions{Sort: SortNatural, IntegrityCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	defer comic.Close()

	problems := comic.Problems()
	if len(problems) != 1 || problems[0].Name != "2.png" {
		t.Fatalf("problemas = %+v, quiero la página 2.png", problems)
	}
}

func TestPrefetchContextFillsCache(t *testing.T) {
	comicPath := writeTestZIP(t, map[string][]byte{
		"1.png": testPNG(t, 4, 4, nil),
		"2.png": testPNG(t, 4, 4, nil),
		"3.png": testPNG(t, 4, 4, nil),
		"4.png": testPNG(t, 4, 4, nil),
	})
	comic, err := OpenComic(comicPath)
	if err != nil {
		t.Fatal(err)
	}
	defer comic.Close()

	if err := comic.PrefetchContext(context.Background(), []string{"2.png", "3.png"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := comic.cache.get("2.png"); !ok {
		t.Fatal("2.png no quedó precargada")
	}
	if _, ok := comic.cache.get("3.png"); !ok {
		t.Fatal("3.png no quedó precargada")
	}
}

func TestZIPIndexCacheRestoresPages(t *testing.T) {
	comicPath := writeTestZIP(t, map[string][]byte{
		"page10.png": testPNG(t, 4, 4, nil),
		"page2.png":  testPNG(t, 4, 4, nil),
	})
	cache := NewDiskCache(t.TempDir(), 1<<20)
	defer cache.Clear()
	opts := OpenOptions{Sort: SortNatural, DiskCache: cache, IntegrityCheck: true}

	first, err := OpenComicWithOptions(comicPath, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]string(nil), first.Pages...)
	if stats := cache.Stats(); stats.Indexes != 1 {
		t.Fatalf("índices guardados = %d, quiero 1", stats.Indexes)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := OpenComicWithOptions(comicPath, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	assertPages(t, second.Pages, want)
	for _, page := range second.Pages {
		if second.zipFiles[page] == nil {
			t.Fatalf("la página %q no se restauró en zipFiles", page)
		}
	}
}

func TestPageCacheStatsAndClear(t *testing.T) {
	cache := newPageCache(1 << 20)
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	cache.put("a", img)
	count, bytes, limit := cache.stats()
	if count != 1 || bytes != int64(len(img.Pix)) || limit != 1<<20 {
		t.Fatalf("stats = %d/%d/%d", count, bytes, limit)
	}
	cache.clear()
	count, bytes, _ = cache.stats()
	if count != 0 || bytes != 0 {
		t.Fatalf("tras clear = %d/%d", count, bytes)
	}
}
