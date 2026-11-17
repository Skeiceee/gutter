package backend

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"

	"github.com/nwaples/rardecode/v2"
	xdraw "golang.org/x/image/draw"
)

// Thumbnail devuelve una miniatura RGBA que cabe en maxSize píxeles por lado.
// Usa una caché separada para no competir con las páginas completas.
func (c *Comic) Thumbnail(imageName string, maxSize int) (*image.RGBA, error) {
	return c.ThumbnailContext(context.Background(), imageName, maxSize)
}

// ThumbnailContext es la variante cancelable de Thumbnail.
func (c *Comic) ThumbnailContext(ctx context.Context, imageName string, maxSize int) (*image.RGBA, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	maxSize = normalizeThumbSize(maxSize)
	cacheKey := thumbnailCacheKey(imageName, maxSize)

	return c.loadImage(ctx, "thumb:"+cacheKey, cacheKey, c.thumbs, func(loadCtx context.Context) (*image.RGBA, error) {
		// La caché en disco evita descomprimir y decodificar la página cuando la
		// rejilla se abre por segunda vez.
		if c.opts.DiskCache != nil && c.diskIdentity != "" {
			if img, ok := c.opts.DiskCache.LoadThumbnail(c.diskIdentity, imageName, maxSize); ok {
				if err := loadCtx.Err(); err != nil {
					return nil, err
				}
				return img, nil
			}
		}

		data, err := c.readEntry(loadCtx, imageName)
		if err != nil {
			return nil, err
		}
		src, err := decodeImage(data, imageName)
		if err != nil {
			return nil, fmt.Errorf("no se pudo decodificar %q para la miniatura: %w", imageName, err)
		}
		thumb := makeThumbnail(src, maxSize)
		c.saveThumbnailToDisk(imageName, maxSize, thumb)
		return thumb, nil
	})
}

// ThumbnailsContext genera varias miniaturas y las entrega en cuanto están
// listas. En RAR sólidos recorre el contenedor una sola vez, igual que la
// precarga de páginas: pedir cada miniatura por separado multiplicaría el
// trabajo del disco.
//
// report se invoca desde la goroutine que llama a este método. La UI debe
// reenviar la actualización a su hilo principal. Un error en una página no
// detiene las demás; solo la cancelación del contexto interrumpe el lote.
func (c *Comic) ThumbnailsContext(ctx context.Context, names []string, maxSize int, report func(string, *image.RGBA, error)) error {
	if c == nil || len(names) == 0 || report == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	maxSize = normalizeThumbSize(maxSize)

	targets := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "" {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}

		cacheKey := thumbnailCacheKey(name, maxSize)
		if img, ok := c.thumbs.get(cacheKey); ok {
			report(name, img, nil)
			continue
		}
		if c.opts.DiskCache != nil && c.diskIdentity != "" {
			if img, ok := c.opts.DiskCache.LoadThumbnail(c.diskIdentity, name, maxSize); ok {
				c.thumbs.put(cacheKey, img)
				report(name, img, nil)
				continue
			}
		}
		targets = append(targets, name)
	}
	if len(targets) == 0 {
		return nil
	}

	if c.zipFile == nil {
		if err := c.ensureRARFiles(); err != nil {
			for _, name := range targets {
				report(name, nil, err)
			}
			return err
		}
		if c.rarSolid {
			return c.thumbnailsRARSequential(ctx, targets, maxSize, report)
		}
	}

	for _, name := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		img, err := c.ThumbnailContext(ctx, name, maxSize)
		if err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		report(name, img, err)
	}
	return nil
}

// thumbnailsRARSequential recorre un RAR sólido una vez y genera solo las
// miniaturas pedidas. Las entradas que no interesan se descartan al avanzar,
// sin decodificarlas.
func (c *Comic) thumbnailsRARSequential(ctx context.Context, names []string, maxSize int, report func(string, *image.RGBA, error)) error {
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
		for name := range targets {
			report(name, nil, err)
		}
		return err
	}
	defer rc.Close()

	for len(targets) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := rc.Next()
		if errors.Is(err, io.EOF) {
			missing := fmt.Errorf("no se encontró la página en el RAR sólido")
			for name := range targets {
				report(name, nil, missing)
			}
			return nil
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			for name := range targets {
				report(name, nil, err)
			}
			return err
		}
		if !c.acceptsRARFile(header) {
			continue
		}
		if _, wanted := targets[header.Name]; !wanted {
			continue
		}
		delete(targets, header.Name)

		data, err := readAllLimited(ctx, rc, header.UnPackedSize)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			report(header.Name, nil, err)
			continue
		}
		src, err := decodeImage(data, header.Name)
		if err != nil {
			report(header.Name, nil, fmt.Errorf("no se pudo decodificar %q para la miniatura: %w", header.Name, err))
			continue
		}
		thumb := makeThumbnail(src, maxSize)
		c.thumbs.put(thumbnailCacheKey(header.Name, maxSize), thumb)
		c.saveThumbnailToDisk(header.Name, maxSize, thumb)
		report(header.Name, thumb, nil)
	}
	return nil
}

func normalizeThumbSize(maxSize int) int {
	if maxSize <= 0 {
		return defaultThumbMaxSize
	}
	if maxSize > 1024 {
		return 1024
	}
	return maxSize
}

func thumbnailCacheKey(imageName string, maxSize int) string {
	return fmt.Sprintf("%s@%d", imageName, maxSize)
}

// makeThumbnail reduce la imagen para que quepa en maxSize píxeles. El
// resultado se guarda en la caché de miniaturas, nunca en la de páginas.
func makeThumbnail(src image.Image, maxSize int) *image.RGBA {
	b := src.Bounds()
	if b.Dx() <= maxSize && b.Dy() <= maxSize {
		return toRGBA(src)
	}

	scale := float64(maxSize) / float64(max(b.Dx(), b.Dy()))
	width := max(1, int(float64(b.Dx())*scale))
	height := max(1, int(float64(b.Dy())*scale))
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// CatmullRom directo sobre una página de 3000 px es muy caro. Cuando el
	// original es mucho mayor que la miniatura se hace una reducción rápida a
	// 2× el tamaño final y solo el último paso usa el filtro de calidad.
	if b.Dx() > maxSize*3 || b.Dy() > maxSize*3 {
		pre := image.NewRGBA(image.Rect(0, 0, width*2, height*2))
		xdraw.ApproxBiLinear.Scale(pre, pre.Bounds(), src, b, draw.Src, nil)
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), pre, pre.Bounds(), draw.Src, nil)
		return dst
	}

	// CatmullRom conserva mejor los trazos y el texto de las páginas que el
	// bilineal aproximado, que dejaba las miniaturas especialmente blandas.
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}

// saveThumbnailToDisk aplica la misma espera de Close que las páginas, pero
// escribe en el presupuesto separado de miniaturas.
func (c *Comic) saveThumbnailToDisk(imageName string, maxSize int, img *image.RGBA) {
	if c.opts.DiskCache == nil || c.diskIdentity == "" || img == nil {
		return
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.diskWG.Add(1)
	c.mu.Unlock()

	go func() {
		defer c.diskWG.Done()
		c.opts.DiskCache.SaveThumbnail(c.diskIdentity, imageName, maxSize, img)
	}()
}
