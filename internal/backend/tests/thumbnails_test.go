package tests

import (
	"context"
	"errors"
	"image"
	"testing"

	"gutter/internal/backend"
)

// ThumbnailsContext debe entregar todas las miniaturas pedidas de un ZIP y
// dejar constancia en el presupuesto separado de la caché en disco.
func TestThumbnailsContextZIPAndDiskCache(t *testing.T) {
	comicPath := writeTestZIP(t, map[string][]byte{
		"page1.png": testPNG(t, 40, 60, nil),
		"page2.png": testPNG(t, 60, 40, nil),
		"page3.png": testPNG(t, 30, 30, nil),
	})
	cache := backend.NewDiskCache(t.TempDir(), 8<<20)
	defer cache.Clear()

	comic, err := backend.OpenComicWithOptions(comicPath, backend.OpenOptions{DiskCache: cache})
	if err != nil {
		t.Fatal(err)
	}

	got := make(map[string]*image.RGBA)
	var failures []string
	err = comic.ThumbnailsContext(context.Background(), comic.Pages, 32, func(name string, img *image.RGBA, err error) {
		if err != nil {
			failures = append(failures, name)
			return
		}
		got[name] = img
	})
	if err != nil {
		t.Fatalf("ThumbnailsContext devolvió %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("miniaturas con error: %v", failures)
	}
	if len(got) != len(comic.Pages) {
		t.Fatalf("miniaturas = %d, quiero %d", len(got), len(comic.Pages))
	}
	for name, img := range got {
		if img == nil {
			t.Fatalf("%q llegó como nil", name)
		}
		if b := img.Bounds(); b.Dx() > 32 || b.Dy() > 32 {
			t.Fatalf("%q mide %v, supera el máximo de 32 px", name, b)
		}
	}

	identity := comic.DiskIdentityForTest()
	firstPage := comic.Pages[0]
	if err := comic.Close(); err != nil {
		t.Fatal(err)
	}

	stats := cache.Stats()
	if stats.Thumbs != len(got) {
		t.Fatalf("miniaturas en disco = %d, quiero %d", stats.Thumbs, len(got))
	}
	if stats.ThumbBytes <= 0 || stats.ThumbLimit <= 0 {
		t.Fatalf("presupuesto de miniaturas inesperado: %+v", stats)
	}
	if _, ok := cache.LoadThumbnail(identity, firstPage, 32); !ok {
		t.Fatal("la miniatura no se recuperó de la caché en disco")
	}
	// El tamaño forma parte de la clave: no debe reutilizarse otra resolución.
	if _, ok := cache.LoadThumbnail(identity, firstPage, 240); ok {
		t.Fatal("una miniatura de otro tamaño no debe reutilizarse")
	}

	// Una segunda apertura debe servirse de la caché persistente sin duplicar
	// entradas en el presupuesto.
	again, err := backend.OpenComicWithOptions(comicPath, backend.OpenOptions{DiskCache: cache})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	if err := again.ThumbnailsContext(context.Background(), again.Pages, 32, func(string, *image.RGBA, error) {
		count++
	}); err != nil {
		t.Fatal(err)
	}
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
	if count != len(again.Pages) {
		t.Fatalf("segunda pasada = %d miniaturas, quiero %d", count, len(again.Pages))
	}
	if stats := cache.Stats(); stats.Thumbs != len(got) {
		t.Fatalf("tras reabrir hay %d miniaturas, quiero %d", stats.Thumbs, len(got))
	}
}

// Un nombre inexistente debe reportarse página a página sin abortar el lote.
func TestThumbnailsContextReportsMissingPage(t *testing.T) {
	comicPath := writeTestZIP(t, map[string][]byte{
		"page1.png": testPNG(t, 20, 20, nil),
	})
	comic, err := backend.OpenComicWithOptions(comicPath, backend.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer comic.Close()

	var reported error
	err = comic.ThumbnailsContext(context.Background(), []string{"missing.png"}, 32, func(_ string, img *image.RGBA, err error) {
		if img != nil {
			t.Error("una página inexistente no debe producir imagen")
		}
		reported = err
	})
	if err != nil {
		t.Fatalf("el lote no debe fallar por una página: %v", err)
	}
	if reported == nil {
		t.Fatal("la página inexistente debe reportar un error")
	}
}

// Cancelar el contexto antes de empezar debe detener el lote sin entregar
// miniaturas a medias.
func TestThumbnailsContextHonorsCancellation(t *testing.T) {
	comicPath := writeTestZIP(t, map[string][]byte{
		"page1.png": testPNG(t, 20, 20, nil),
	})
	comic, err := backend.OpenComicWithOptions(comicPath, backend.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer comic.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	reported := 0
	err = comic.ThumbnailsContext(ctx, comic.Pages, 32, func(string, *image.RGBA, error) {
		reported++
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, quiero context.Canceled", err)
	}
	if reported != 0 {
		t.Fatalf("se entregaron %d miniaturas tras cancelar", reported)
	}
}

func TestThumbDiskBudget(t *testing.T) {
	cases := []struct {
		pageLimit int64
		want      int64
	}{
		{8 << 20, backend.MinThumbDiskLimit},
		{256 << 20, 64 << 20},
		{512 << 20, 128 << 20},
		{4 << 30, backend.MaxThumbDiskLimit},
	}
	for _, tc := range cases {
		cache := backend.NewDiskCache(t.TempDir(), tc.pageLimit)
		if got := cache.Stats().ThumbLimit; got != tc.want {
			t.Errorf("límite de páginas %d: miniaturas %d, quiero %d", tc.pageLimit, got, tc.want)
		}
		cache.Clear()
	}
}

// Las miniaturas y las páginas comparten formato pero no directorio ni
// contadores: guardar una no debe alterar el presupuesto de la otra.
func TestDiskCacheThumbnailRoundTripIsIsolated(t *testing.T) {
	cache := backend.NewDiskCache(t.TempDir(), 8<<20)
	defer cache.Clear()

	img := image.NewRGBA(image.Rect(0, 0, 8, 6))
	cache.SaveThumbnail("comic-1", "page.png", 120, img)

	stats := cache.Stats()
	if stats.Thumbs != 1 || stats.ThumbBytes <= 0 {
		t.Fatalf("miniaturas = %+v", stats)
	}
	if stats.Pages != 0 || stats.PageBytes != 0 {
		t.Fatalf("guardar una miniatura no debe contar como página: %+v", stats)
	}

	got, ok := cache.LoadThumbnail("comic-1", "page.png", 120)
	if !ok {
		t.Fatal("la miniatura no se recuperó")
	}
	if got.Bounds() != img.Bounds() {
		t.Fatalf("bounds = %v, quiero %v", got.Bounds(), img.Bounds())
	}

	cache.Clear()
	if stats := cache.Stats(); stats.Thumbs != 0 || stats.ThumbBytes != 0 {
		t.Fatalf("contadores tras Clear = %+v", stats)
	}
}
