package tests

import (
	"fmt"
	"image"
	"sort"
	"testing"

	"gutter/internal/backend"
)

func BenchmarkPageCacheLRU(b *testing.B) {
	const count = 128
	images := make([]*image.RGBA, count)
	names := make([]string, count)
	for i := range images {
		images[i] = image.NewRGBA(image.Rect(0, 0, 256, 256))
		names[i] = fmt.Sprintf("page-%03d.jpg", i)
	}

	// Presupuesto pequeño a propósito para medir también la evicción.
	cache := backend.NewPageCache(8 << 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index := i % count
		cache.Put(names[index], images[index])
		_, _ = cache.Get(names[(index*7)%count])
	}
}

func BenchmarkNaturalOrder(b *testing.B) {
	const count = 1000
	names := make([]string, count)
	for i := range names {
		names[i] = fmt.Sprintf("Chapter 1/page-%d.jpg", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pages := append([]string(nil), names...)
		sort.SliceStable(pages, func(i, j int) bool {
			return backend.NaturalLess(pages[i], pages[j])
		})
	}
}

func BenchmarkSpreadDetection(b *testing.B) {
	portrait := image.NewRGBA(image.Rect(0, 0, 1000, 1500))
	landscape := image.NewRGBA(image.Rect(0, 0, 2000, 1500))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			_ = backend.IsLandscape(portrait)
		} else {
			_ = backend.IsLandscape(landscape)
		}
	}
}

func BenchmarkCropHalf(b *testing.B) {
	page := image.NewRGBA(image.Rect(0, 0, 1600, 2400))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backend.CropHalf(page, i%2 == 0)
	}
}

func BenchmarkMakeThumbnail(b *testing.B) {
	page := image.NewRGBA(image.Rect(0, 0, 1600, 2400))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backend.MakeThumbnail(page, 160)
	}
}
