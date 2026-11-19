package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"

	"github.com/nwaples/rardecode/v2"

	// Decodificadores registrados de forma nativa en image.Decode.
	//
	// AVIF es puro en Go (libavif/dav1d vía WASM, sin cgo).
	_ "github.com/gen2brain/avif"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// ReadComicIndex devuelve los nombres de las imágenes internas ordenados.
// Es la función de conveniencia para listar un archivo sin abrir sesión.
func ReadComicIndex(comicPath string) ([]string, error) {
	comic, err := OpenComic(comicPath)
	if err != nil {
		return nil, err
	}
	defer comic.Close()

	pages := make([]string, len(comic.Pages))
	copy(pages, comic.Pages)
	return pages, nil
}

// GetPage es la API de una sola llamada solicitada por el diseño de Gutter.
//
// Abre el archivo, localiza exactamente imageName, lo decodifica en memoria y,
// si singlePageView está activo y la página es horizontal, devuelve una de sus
// mitades usando SubImage.
//
// Para navegar dentro de la UI conviene reutilizar una sesión *Comic y su
// método GetPage: así se comparte la caché y el prefetch entre páginas.
func GetPage(comicPath string, imageName string, singlePageView bool, getRightHalf bool) (image.Image, error) {
	comic, err := OpenComic(comicPath)
	if err != nil {
		return nil, err
	}
	defer comic.Close()

	return comic.GetPage(imageName, singlePageView, getRightHalf)
}

// GetPage devuelve una página de esta sesión, aplicando el corte virtual.
func (c *Comic) GetPage(imageName string, singlePageView bool, getRightHalf bool) (image.Image, error) {
	return c.GetPageContext(context.Background(), imageName, singlePageView, getRightHalf)
}

// GetPageContext es la variante cancelable de GetPage.
func (c *Comic) GetPageContext(ctx context.Context, imageName string, singlePageView bool, getRightHalf bool) (image.Image, error) {
	full, err := c.PageContext(ctx, imageName)
	if err != nil {
		return nil, err
	}

	if singlePageView && isLandscape(full) {
		return CropHalf(full, getRightHalf), nil
	}
	return full, nil
}

// Page devuelve la página completa como *image.RGBA, usando la caché LRU.
func (c *Comic) Page(imageName string) (*image.RGBA, error) {
	return c.PageContext(context.Background(), imageName)
}

// PageContext es la versión cancelable de Page. La imagen se carga en una
// goroutine con single-flight; si todos los que esperan cancelan, la lectura se
// cancela y no se desperdicia I/O.
func (c *Comic) PageContext(ctx context.Context, imageName string) (*image.RGBA, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return c.loadImage(ctx, "page:"+imageName, imageName, c.cache, func(loadCtx context.Context) (*image.RGBA, error) {
		return c.loadPage(loadCtx, imageName)
	})
}

// IsSpread indica si una página es horizontal (doble página).
func (c *Comic) IsSpread(imageName string) (bool, error) {
	return c.IsSpreadContext(context.Background(), imageName)
}

// IsSpreadContext es la variante cancelable de IsSpread.
func (c *Comic) IsSpreadContext(ctx context.Context, imageName string) (bool, error) {
	img, err := c.PageContext(ctx, imageName)
	if err != nil {
		return false, err
	}
	return isLandscape(img), nil
}

// CropHalf recorta la mitad izquierda o derecha de una página doble.
//
// El recorte se define con SubImage, que es O(1) y comparte el buffer. Sin
// embargo, se compacta a un *image.RGBA nuevo antes de devolverlo: SubImage
// conserva el Stride de la página completa y el cargador OpenGL de Fyne sube
// los píxeles de forma contigua (asume Stride = ancho*4). Entregar el subimage
// directamente produciría filas desplazadas. La copia es de media página y solo
// ocurre al mostrar una doble página partida.
func CropHalf(img *image.RGBA, getRightHalf bool) image.Image {
	b := img.Bounds()
	mid := b.Min.X + b.Dx()/2

	var half image.Rectangle
	if getRightHalf {
		half = image.Rect(mid, b.Min.Y, b.Max.X, b.Max.Y)
	} else {
		half = image.Rect(b.Min.X, b.Min.Y, mid, b.Max.Y)
	}

	sub := img.SubImage(half)
	out := image.NewRGBA(image.Rect(0, 0, sub.Bounds().Dx(), sub.Bounds().Dy()))
	draw.Draw(out, out.Bounds(), sub, sub.Bounds().Min, draw.Src)
	return out
}

// loadPage extrae y decodifica una entrada concreta. Antes de tocar el
// contenedor intenta recuperar la página de la caché en disco.
func (c *Comic) loadPage(ctx context.Context, imageName string) (*image.RGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.opts.DiskCache != nil && c.diskIdentity != "" {
		if img, ok := c.opts.DiskCache.LoadPage(c.diskIdentity, imageName); ok {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return img, nil
		}
	}

	data, err := c.readEntry(ctx, imageName)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	src, err := decodeImage(data, imageName)
	if err != nil {
		return nil, fmt.Errorf("no se pudo decodificar %q: %w", imageName, err)
	}

	// Normalización obligatoria: GIF animado, WebP, AVIF y algunos PNG
	// devuelven image.Paletted/NRGBA/YCbCr. Fyne y SubImage necesitan RGBA.
	return toRGBA(src), nil
}

// PersistPage guarda en la caché de disco la página indicada si ya está
// decodificada en RAM. Se llama solo con la página que el lector muestra: el
// prefetch se queda en memoria y no genera escrituras de varios MiB por cada
// cambio de página.
func (c *Comic) PersistPage(imageName string) {
	if c == nil || imageName == "" || c.opts.DiskCache == nil || c.diskIdentity == "" {
		return
	}
	if c.opts.DiskCache.HasPage(c.diskIdentity, imageName) {
		return
	}
	if img, ok := c.cache.get(imageName); ok {
		c.savePageToDisk(imageName, img)
	}
}

// savePageToDisk escribe la página en segundo plano. Close espera a que
// terminen las escrituras pendientes para no dejar archivos a medias.
func (c *Comic) savePageToDisk(imageName string, img *image.RGBA) {
	if c.opts.DiskCache == nil || c.diskIdentity == "" || img == nil {
		return
	}
	// Add se hace con c.mu tomado y Close marca closed antes de esperar. Así
	// ninguna escritura puede empezar después de que Close haya medido el
	// WaitGroup.
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.diskWG.Add(1)
	c.mu.Unlock()

	go func() {
		defer c.diskWG.Done()
		c.opts.DiskCache.SavePage(c.diskIdentity, imageName, img)
	}()
}

// readEntry obtiene los bytes comprimidos de una entrada del ZIP o del RAR.
func (c *Comic) readEntry(ctx context.Context, imageName string) ([]byte, error) {
	if c.zipFile != nil {
		return c.readZIPEntry(ctx, imageName)
	}
	return c.readRAREntry(ctx, imageName)
}

func (c *Comic) readZIPEntry(ctx context.Context, imageName string) ([]byte, error) {
	f := c.zipFiles[imageName]
	if f == nil {
		return nil, fmt.Errorf("la página %q no existe en el ZIP", imageName)
	}
	if f.UncompressedSize64 > uint64(maxPageBytes) {
		return nil, fmt.Errorf("la imagen declara %d MiB, por encima del límite de %d MiB", f.UncompressedSize64>>20, maxPageBytes>>20)
	}

	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %q del ZIP: %w", imageName, err)
	}
	defer rc.Close()

	return readAllLimited(ctx, rc, int64(f.UncompressedSize64))
}

// readRAREntry usa acceso aleatorio cuando el RAR no es sólido y acceso
// secuencial como fallback.
func (c *Comic) readRAREntry(ctx context.Context, imageName string) ([]byte, error) {
	if err := c.ensureRARFiles(); err != nil {
		return nil, err
	}
	f := c.rarFiles[imageName]
	if f == nil {
		return nil, fmt.Errorf("la página %q no existe en el RAR", imageName)
	}

	var openErr error
	if !f.Solid {
		// Acceso aleatorio: rardecode reabre el volumen y salta al bloque de
		// esta entrada sin descomprimir las anteriores.
		rc, err := f.Open()
		if err == nil {
			defer rc.Close()
			data, readErr := readAllLimited(ctx, rc, f.UnPackedSize)
			if readErr != nil {
				return nil, fmt.Errorf("no se pudo leer %q: %w", imageName, readErr)
			}
			return data, nil
		}
		openErr = err
	}

	// Los archivos sólidos (y algunos volúmenes dañados) solo permiten leer en
	// orden. Es más lento, pero la caché evita repetirlo al volver atrás.
	data, err := c.readSequential(ctx, imageName)
	if err != nil {
		if openErr != nil {
			return nil, fmt.Errorf("no se pudo extraer %q: %v; acceso secuencial: %w", imageName, openErr, err)
		}
		return nil, fmt.Errorf("no se pudo extraer %q: %w", imageName, err)
	}
	return data, nil
}

// ensureRARFiles completa el mapa de metadatos de un RAR abierto desde la
// caché de índice. Solo se llama cuando hace falta leer una página: la
// apertura inicial sigue siendo instantánea.
func (c *Comic) ensureRARFiles() error {
	c.rarMu.Lock()
	defer c.rarMu.Unlock()

	if c.rarReady {
		return nil
	}
	files, err := rardecode.List(c.Path)
	if err != nil {
		return fmt.Errorf("no se pudo releer el índice del RAR %q: %w", c.Path, err)
	}

	wanted := make(map[string]struct{}, len(c.Pages))
	for _, page := range c.Pages {
		wanted[page] = struct{}{}
	}
	for _, f := range files {
		if _, ok := wanted[f.Name]; !ok {
			continue
		}
		if f.IsDir || f.LinkType != rardecode.LinkTypeNone {
			continue
		}
		c.rarFiles[f.Name] = f
		if f.Solid {
			c.rarSolid = true
		}
	}
	c.rarReady = true
	return nil
}

// ReadProgressKey transporta un callback de progreso dentro del contexto de
// carga. Solo lo usa la página actual; el prefetch no lo necesita.
type ReadProgressKey struct{}

type ReadProgressFunc func(scanned, total int)

func WithReadProgress(ctx context.Context, fn ReadProgressFunc) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ReadProgressKey{}, fn)
}

func reportReadProgress(ctx context.Context, scanned, total int) {
	if ctx == nil {
		return
	}
	if fn, ok := ctx.Value(ReadProgressKey{}).(ReadProgressFunc); ok && fn != nil {
		fn(scanned, total)
	}
}

// readSequential recorre el RAR hasta encontrar imageName, informando del
// avance para que la UI pueda mostrar "RAR sólido 120/455".
func (c *Comic) readSequential(ctx context.Context, imageName string) ([]byte, error) {
	rc, err := rardecode.OpenReader(c.Path)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	total := len(c.Pages)
	scanned := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := rc.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("no se encontró %q", imageName)
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, err
		}
		if !c.acceptsRARFile(header) {
			continue
		}

		scanned++
		if scanned == 1 || scanned%10 == 0 || header.Name == imageName {
			reportReadProgress(ctx, scanned, total)
		}
		if header.Name != imageName {
			continue
		}

		data, err := readAllLimited(ctx, rc, header.UnPackedSize)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
}

// contextReader hace que io.ReadAll aborte en cuanto se cancela la carga.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if err == nil {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			return n, ctxErr
		}
	}
	return n, err
}

// readAllLimited lee una entrada aplicando un límite de seguridad.
func readAllLimited(ctx context.Context, r io.Reader, unpackedSize int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if unpackedSize > maxPageBytes {
		return nil, fmt.Errorf("la imagen declara %d MiB, por encima del límite de %d MiB", unpackedSize>>20, maxPageBytes>>20)
	}

	data, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, r: r}, maxPageBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxPageBytes {
		return nil, fmt.Errorf("la imagen supera el límite de %d MiB", maxPageBytes>>20)
	}
	return data, nil
}

// decodeImage decodifica una entrada respetando maxPagePixels. Primero lee solo
// la cabecera: así una imagen que declara dimensiones absurdas se rechaza antes
// de que el decodificador reserve memoria para todos sus píxeles.
func decodeImage(data []byte, name string) (image.Image, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("la imagen %q declara dimensiones no válidas (%dx%d)", name, cfg.Width, cfg.Height)
	}
	// Comprobar cada factor evita el desbordamiento de int64 con dimensiones
	// absurdas (2^32-1 × 2^32-1 daría un producto negativo).
	if int64(cfg.Width) > maxPagePixels || int64(cfg.Height) > maxPagePixels ||
		int64(cfg.Width)*int64(cfg.Height) > maxPagePixels {
		return nil, fmt.Errorf(
			"la imagen %q (%s) declara %dx%d píxeles, por encima del límite de %d megapíxeles",
			name, format, cfg.Width, cfg.Height, maxPagePixels/1_000_000,
		)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

// toRGBA convierte cualquier image.Image a *image.RGBA con origen (0,0).
// Si ya es RGBA canónica evita la copia.
func toRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok && rgba.Bounds().Min == (image.Point{}) {
		return rgba
	}

	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// isLandscape define "página doble": más ancha que alta.
func isLandscape(img image.Image) bool {
	b := img.Bounds()
	return b.Dx() > b.Dy()
}
