package tests

// memprobe_test.go es un banco de medición manual, no una prueba automática: se
// salta salvo que se defina GUTTER_COMIC. Sirve para comparar el consumo de
// memoria antes y después de un cambio:
//
//	$env:GUTTER_COMIC = "C:\ruta\al\comic.cbr"
//	$env:GUTTER_DISK_DIR = ""   # opcional: ruta para medir con caché en disco
//	go test -run TestMemProbe -v ./internal/backend/tests/
//
// Imprime el heap final, el pico observado cada 5 ms y el tamaño de las cachés
// tras indexar, leer, precargar, cambiar de página y generar miniaturas.

import (
	"context"
	"fmt"
	"image"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"gutter/internal/backend"
)

func mb(v uint64) float64 { return float64(v) / (1 << 20) }

func TestMemProbe(t *testing.T) {
	comicPath := os.Getenv("GUTTER_COMIC")
	if comicPath == "" {
		t.Skip("GUTTER_COMIC no definido")
	}

	var peak atomic.Uint64
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				if m.HeapAlloc > peak.Load() {
					peak.Store(m.HeapAlloc)
				}
			}
		}
	}()
	defer close(stop)

	measure := func(label string, fn func()) {
		peak.Store(0)
		fn()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Printf("%-26s fin=%7.1f MB  pico=%7.1f MB  sys=%7.1f MB\n",
			label, mb(m.HeapAlloc), mb(peak.Load()), mb(m.Sys))
	}

	var comic *backend.Comic
	var err error
	measure("indexar", func() {
		opts := backend.OpenOptions{Sort: backend.SortNatural}
		if dir := os.Getenv("GUTTER_DISK_DIR"); dir != "" {
			opts.DiskCache = backend.NewDiskCache(dir, 512<<20)
		}
		comic, err = backend.OpenComicWithOptions(comicPath, opts)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer comic.Close()
	fmt.Printf("páginas=%d solid=%v\n", len(comic.Pages), comic.IsSolid())

	ctx := context.Background()
	printedSizes := 0
	measure("cargar 8 páginas", func() {
		for i := 0; i < 8 && i < len(comic.Pages); i++ {
			img, err := comic.PageContext(ctx, comic.Pages[i])
			if err != nil {
				t.Fatal(err)
			}
			if printedSizes < 3 {
				printedSizes++
				fmt.Printf("  pág %d: %d×%d = %.2f MB RGBA\n",
					i, img.Bounds().Dx(), img.Bounds().Dy(), float64(len(img.Pix))/(1<<20))
			}
		}
		printCache(t, comic)
	})

	measure("prefetch 2 siguientes + 1", func() {
		end := 8 + 2
		if end > len(comic.Pages) {
			end = len(comic.Pages)
		}
		names := append([]string(nil), comic.Pages[8:end]...)
		if len(comic.Pages) > 0 {
			names = append(names, comic.Pages[max(0, 7)])
		}
		if err := comic.PrefetchContext(ctx, names); err != nil {
			t.Fatal(err)
		}
		printCache(t, comic)
	})

	measure("20 cambios de página", func() {
		for i := 0; i < 20 && i+3 < len(comic.Pages); i++ {
			if _, err := comic.PageContext(ctx, comic.Pages[i]); err != nil {
				t.Fatal(err)
			}
			comic.PersistPage(comic.Pages[i])
			if err := comic.PrefetchContext(ctx, comic.Pages[i+1:i+3]); err != nil {
				t.Fatal(err)
			}
		}
		printCache(t, comic)
	})

	measure("miniaturas", func() {
		comic.ClearMemoryCaches()
		count := 0
		if err := comic.ThumbnailsContext(ctx, comic.Pages, 160, func(name string, img *image.RGBA, err error) {
			count++
		}); err != nil {
			t.Fatal(err)
		}
		fmt.Printf("  miniaturas generadas: %d\n", count)
		printCache(t, comic)
	})

	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("%-26s fin=%7.1f MB  sys=%7.1f MB\n", "tras cerrar", mb(m.HeapAlloc), mb(m.Sys))
}

func printCache(t *testing.T, c *backend.Comic) {
	t.Helper()
	s := c.MemoryStats()
	fmt.Printf("  caché: %d pág %.1f/%.1f MB | %d miniaturas %.1f/%.1f MB\n",
		s.PageCount, float64(s.PageBytes)/(1<<20), float64(s.PageLimit)/(1<<20),
		s.ThumbCount, float64(s.ThumbBytes)/(1<<20), float64(s.ThumbLimit)/(1<<20))
}
