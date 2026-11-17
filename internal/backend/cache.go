package backend

import (
	"container/list"
	"image"
	"sync"
)

// pageCache es una LRU acotada en bytes para imágenes RGBA. Se instancia dos
// veces: para páginas completas y para miniaturas, con presupuestos separados.
type pageCache struct {
	mu       sync.Mutex
	maxBytes int64
	bytes    int64
	order    *list.List
	items    map[string]*list.Element
}

type cachedPage struct {
	name string
	img  *image.RGBA
}

func newPageCache(maxBytes int64) *pageCache {
	return &pageCache{
		maxBytes: maxBytes,
		order:    list.New(),
		items:    make(map[string]*list.Element),
	}
}

func (c *pageCache) get(name string) (*image.RGBA, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[name]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*cachedPage).img, true
}

func (c *pageCache) put(name string, img *image.RGBA) {
	if img == nil {
		return
	}
	size := int64(len(img.Pix))
	if size > c.maxBytes {
		return // una sola imagen no cabe en el presupuesto
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[name]; ok {
		entry := el.Value.(*cachedPage)
		c.bytes += size - int64(len(entry.img.Pix))
		entry.img = img
		c.order.MoveToFront(el)
	} else {
		el := c.order.PushFront(&cachedPage{name: name, img: img})
		c.items[name] = el
		c.bytes += size
	}

	for c.bytes > c.maxBytes && c.order.Len() > 1 {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		entry := oldest.Value.(*cachedPage)
		c.order.Remove(oldest)
		delete(c.items, entry.name)
		c.bytes -= int64(len(entry.img.Pix))
	}
}

// stats devuelve número de elementos y bytes usados, junto al presupuesto.
func (c *pageCache) stats() (count int, bytes, maxBytes int64) {
	if c == nil {
		return 0, 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len(), c.bytes, c.maxBytes
}

// clear libera todas las imágenes retenidas por la caché.
func (c *pageCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.order.Init()
	c.items = make(map[string]*list.Element)
	c.bytes = 0
}
