package backend

import (
	"context"
	"errors"
	"io"

	"github.com/nwaples/rardecode/v2"
)

// PrefetchContext precarga varias páginas en la dirección de lectura. En RAR
// sólidos recorre el contenedor una sola vez para todas ellas; en el resto de
// casos reutiliza la caché y el single-flight normales.
func (c *Comic) PrefetchContext(ctx context.Context, names []string) error {
	if c == nil || len(names) == 0 || c.cache == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	targets := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		if _, cached := c.cache.get(name); cached {
			continue
		}
		targets = append(targets, name)
	}
	if len(targets) == 0 {
		return nil
	}

	if c.zipFile == nil {
		if err := c.ensureRARFiles(); err != nil {
			return err
		}
		if c.rarSolid {
			return c.prefetchRARSequential(ctx, targets)
		}
	}

	for _, name := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := c.PageContext(ctx, name); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			// Un prefetch es best-effort: una página rota no debe detener las
			// demás.
			continue
		}
	}
	return nil
}

// prefetchRARSequential recorre un RAR sólido una vez y guarda todas las
// páginas pedidas que encuentre. Evita repetir el recorrido por cada página.
func (c *Comic) prefetchRARSequential(ctx context.Context, names []string) error {
	if !c.beginScan() {
		return errComicClosed
	}
	defer c.scanWG.Done()

	ctx, cancel := c.scanContext(ctx)
	defer cancel()

	targets := make(map[string]struct{}, len(names))
	for _, name := range names {
		targets[name] = struct{}{}
	}

	rc, err := rardecode.OpenReader(c.Path)
	if err != nil {
		return err
	}
	defer rc.Close()

	for len(targets) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := rc.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return err
		}
		if !c.acceptsRARFile(header) {
			continue
		}
		if _, wanted := targets[header.Name]; !wanted {
			continue
		}

		data, err := readAllLimited(ctx, rc, header.UnPackedSize)
		if err != nil {
			delete(targets, header.Name)
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			continue
		}
		src, err := decodeImage(data, header.Name)
		if err != nil {
			delete(targets, header.Name)
			continue
		}
		img := toRGBA(src)
		c.cache.put(header.Name, img)
		c.savePageToDisk(header.Name, img)
		delete(targets, header.Name)
	}
	return nil
}
