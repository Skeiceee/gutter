package backend

import (
	"archive/zip"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func newLoadTestComic() *Comic {
	return &Comic{
		cache:    newPageCache(defaultPageCacheBudget),
		thumbs:   newPageCache(defaultThumbCacheBudget),
		inflight: make(map[string]*loadCall),
	}
}

func TestCloseWaitsForInFlightLoad(t *testing.T) {
	c := newLoadTestComic()
	started := make(chan struct{})
	canceled := make(chan struct{})

	go func() {
		_, _ = c.loadImage(context.Background(), "page:p", "p", c.cache, func(ctx context.Context) (*image.RGBA, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			return nil, ctx.Err()
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("la carga no llegó a empezar")
	}

	done := make(chan struct{})
	go func() {
		_ = c.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close no esperó a que terminara la lectura")
	}
	select {
	case <-canceled:
	default:
		t.Fatal("Close no canceló la lectura en curso")
	}

	if _, err := c.PageContext(context.Background(), "p"); err == nil {
		t.Fatal("un cómic cerrado no debe aceptar cargas nuevas")
	}
}

func TestCancelLoadsAllowsRestart(t *testing.T) {
	c := newLoadTestComic()
	var calls atomic.Int32
	firstStarted := make(chan struct{})

	loader := func(ctx context.Context) (*image.RGBA, error) {
		if calls.Add(1) == 1 {
			close(firstStarted)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return image.NewRGBA(image.Rect(0, 0, 2, 2)), nil
	}

	firstErr := make(chan error, 1)
	go func() {
		_, err := c.loadImage(context.Background(), "page:p", "p", c.cache, loader)
		firstErr <- err
	}()

	<-firstStarted
	c.CancelLoads()

	select {
	case <-firstErr:
	case <-time.After(2 * time.Second):
		t.Fatal("la primera carga no terminó tras CancelLoads")
	}

	img, err := c.loadImage(context.Background(), "page:p", "p", c.cache, loader)
	if err != nil {
		t.Fatalf("la carga nueva falló tras cancelar la anterior: %v", err)
	}
	if img == nil {
		t.Fatal("la carga nueva no devolvió imagen")
	}
	if calls.Load() != 2 {
		t.Fatalf("llamadas al loader = %d, quiero 2", calls.Load())
	}
}

func TestMakeThumbnailScalesAndKeepsAspect(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			src.Set(x, y, color.RGBA{R: 0x20, G: 0x80, B: 0xC0, A: 0xFF})
		}
	}

	thumb := makeThumbnail(src, 64)
	if got := thumb.Bounds(); got.Dx() != 64 || got.Dy() != 32 {
		t.Fatalf("miniatura = %v, quiero 64x32", got)
	}
	if _, _, _, a := thumb.At(10, 10).RGBA(); a == 0 {
		t.Fatal("la miniatura no conserva los píxeles")
	}

	same := makeThumbnail(src, 256)
	if same.Bounds() != src.Bounds() {
		t.Fatalf("una imagen pequeña no debe ampliarse: %v", same.Bounds())
	}
}

func TestThumbnailUsesSeparateCache(t *testing.T) {
	comicPath := filepath.Join(t.TempDir(), "thumbs.cbz")
	f, err := os.Create(comicPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("page.png")
	if err != nil {
		t.Fatal(err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	if err := png.Encode(w, src); err != nil {
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

	thumb, err := comic.Thumbnail("page.png", 64)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	if thumb.Bounds().Dx() != 64 {
		t.Fatalf("miniatura = %v", thumb.Bounds())
	}
	if _, ok := comic.thumbs.get("page.png@64"); !ok {
		t.Fatal("la miniatura no se guardó en su caché")
	}
	if _, ok := comic.cache.get("page.png"); ok {
		t.Fatal("la miniatura no debe ocupar la caché de páginas completas")
	}
}

func TestLoadImagePreservesProgressContext(t *testing.T) {
	c := newLoadTestComic()
	gotScanned, gotTotal := 0, 0
	ctx := WithReadProgress(context.Background(), func(scanned, total int) {
		gotScanned, gotTotal = scanned, total
	})

	_, err := c.loadImage(ctx, "page:p", "p", c.cache, func(loadCtx context.Context) (*image.RGBA, error) {
		reportReadProgress(loadCtx, 7, 42)
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotScanned != 7 || gotTotal != 42 {
		t.Fatalf("el contexto de progreso no llegó al loader: %d/%d", gotScanned, gotTotal)
	}
}

func TestReadProgressContext(t *testing.T) {
	scanned, total := 0, 0
	ctx := WithReadProgress(context.Background(), func(s, t int) {
		scanned, total = s, t
	})
	reportReadProgress(ctx, 120, 455)
	if scanned != 120 || total != 455 {
		t.Fatalf("progreso = %d/%d", scanned, total)
	}
	reportReadProgress(context.Background(), 1, 1)
	if scanned != 120 {
		t.Fatal("un contexto sin callback no debe actualizar el progreso")
	}
}
