package backend

import (
	"context"
	"errors"
	"image"
)

// loadImage implementa la caché LRU y el single-flight compartido por páginas y
// miniaturas. Si todos los llamadores cancelan, se cancela el trabajo real.
func (c *Comic) loadImage(
	ctx context.Context,
	inflightKey string,
	cacheKey string,
	cache *pageCache,
	loader func(context.Context) (*image.RGBA, error),
) (*image.RGBA, error) {
	if cache == nil {
		return nil, errors.New("la caché de imágenes no está inicializada")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return nil, errComicClosed
	}

	if img, ok := cache.get(cacheKey); ok {
		return img, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errComicClosed
	}
	if call, ok := c.inflight[inflightKey]; ok {
		call.refs++
		c.mu.Unlock()
		return c.waitLoad(ctx, call)
	}

	// WithoutCancel conserva el callback de progreso del llamador, pero la
	// cancelación se controla con el contador de referencias: si otro prefetch
	// sigue esperando, la lectura compartida no debe morir con el primero.
	loadCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	call := &loadCall{
		key:    inflightKey,
		done:   make(chan struct{}),
		refs:   1,
		cancel: cancel,
	}
	c.inflight[inflightKey] = call
	c.loadWG.Add(1)
	c.mu.Unlock()

	go func() {
		img, err := loader(loadCtx)
		if err == nil && img != nil {
			cache.put(cacheKey, img)
		}

		c.mu.Lock()
		call.img, call.err = img, err
		if c.inflight[inflightKey] == call {
			delete(c.inflight, inflightKey)
		}
		close(call.done)
		c.mu.Unlock()
		c.loadWG.Done()
	}()

	return c.waitLoad(ctx, call)
}

func (c *Comic) waitLoad(ctx context.Context, call *loadCall) (*image.RGBA, error) {
	select {
	case <-call.done:
		c.releaseLoad(call)
		return call.img, call.err
	case <-ctx.Done():
		c.releaseLoad(call)
		return nil, ctx.Err()
	}
}

func (c *Comic) releaseLoad(call *loadCall) {
	c.mu.Lock()
	if call.refs > 0 {
		call.refs--
	}
	if call.refs == 0 {
		call.cancel()
		if call.key != "" && c.inflight[call.key] == call {
			delete(c.inflight, call.key)
		}
	}
	c.mu.Unlock()
}
