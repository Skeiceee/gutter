package backend

// testapi.go expone los identificadores internos que necesita el paquete de
// pruebas externalizado en internal/backend/tests.

import (
	"context"
	"image"
	"os"
)

const (
	MinThumbDiskLimit = minThumbDiskLimit
	MaxThumbDiskLimit = maxThumbDiskLimit
)

func NaturalLess(a, b string) bool                           { return naturalLess(a, b) }
func NaturalCompare(a, b string) int                         { return naturalCompare(a, b) }
func ToRGBA(img image.Image) *image.RGBA                     { return toRGBA(img) }
func IsComicImage(name string) bool                          { return isComicImage(name) }
func IsHiddenComicPath(name string) bool                     { return isHiddenComicPath(name) }
func MakeThumbnail(src image.Image, maxSize int) *image.RGBA { return makeThumbnail(src, maxSize) }
func IsLandscape(img image.Image) bool                       { return isLandscape(img) }
func ArchiveIdentity(path string, info os.FileInfo) string   { return archiveIdentity(path, info) }
func ReportReadProgress(ctx context.Context, scanned, total int) {
	reportReadProgress(ctx, scanned, total)
}

// NewLoadTestComic crea un cómic mínimo con las cachés en memoria listas.
func NewLoadTestComic() *Comic {
	return &Comic{
		cache:    newPageCache(defaultPageCacheBudget),
		thumbs:   newPageCache(defaultThumbCacheBudget),
		inflight: make(map[string]*loadCall),
	}
}

// LoadImageForTest llama a la carga single-flight interna usando la caché de
// páginas del propio cómic.
func (c *Comic) LoadImageForTest(
	ctx context.Context,
	inflightKey, cacheKey string,
	loader func(context.Context) (*image.RGBA, error),
) (*image.RGBA, error) {
	return c.loadImage(ctx, inflightKey, cacheKey, c.cache, loader)
}

func (c *Comic) PageCached(name string) bool {
	_, ok := c.cache.get(name)
	return ok
}

func (c *Comic) ThumbnailCached(name string) bool {
	_, ok := c.thumbs.get(name)
	return ok
}

func (c *Comic) DiskIdentityForTest() string { return c.diskIdentity }

func (c *Comic) ZipFilePresent(name string) bool { return c.zipFiles[name] != nil }

// PageCache envuelve pageCache para los benchmarks y tests de la LRU.
type PageCache struct{ cache *pageCache }

func NewPageCache(maxBytes int64) *PageCache { return &PageCache{cache: newPageCache(maxBytes)} }

func (p *PageCache) Put(name string, img *image.RGBA) { p.cache.put(name, img) }

func (p *PageCache) Get(name string) (*image.RGBA, bool) { return p.cache.get(name) }

func (p *PageCache) Stats() (int, int64, int64) { return p.cache.stats() }

func (p *PageCache) Clear() { p.cache.clear() }
