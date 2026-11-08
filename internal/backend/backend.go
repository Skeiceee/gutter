// Package main
//
// backend.go contiene el motor de lectura de cómics de Gutter.
//
// Principios de diseño:
//   - Descompresión selectiva: se lee únicamente la entrada solicitada.
//   - Contenedores RAR v4/v5 y ZIP (detectados por magic bytes, no por extensión).
//   - Memoria acotada: las páginas decodificadas viven en una caché LRU con
//     presupuesto en bytes; nunca se descomprime el cómic completo.
//   - Tipos seguros para Fyne: toda imagen se normaliza a *image.RGBA para que
//     SubImage() esté siempre disponible, incluso en WebP/GIF indexados.
//
// El paquete rardecode/v2 es 100 % Go y soporta RAR v4 y v5, por lo que Gutter
// no necesita WinRAR, unrar.dll ni ningún binario externo.
package backend

import (
	"archive/zip"
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"unicode"

	xdraw "golang.org/x/image/draw"

	// Decodificadores registrados de forma nativa en image.Decode.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "github.com/gen2brain/avif" // AVIF puro en Go (libavif/dav1d vía WASM, sin cgo)
	"github.com/nwaples/rardecode/v2"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	// defaultPageCacheBudget es el presupuesto de RAM para páginas ya
	// decodificadas (RGBA). Con scans de ~20 MB caben 4-5 páginas, suficiente
	// para la página actual, la siguiente (prefetch) y volver atrás sin releer.
	defaultPageCacheBudget int64 = 96 << 20 // 96 MiB

	// maxPageBytes protege contra entradas corruptas o maliciosas que declaren
	// un tamaño desmesurado. Una página de cómic rara vez supera los 256 MiB.
	maxPageBytes int64 = 256 << 20

	// defaultThumbCacheBudget es independiente de la caché de páginas: las
	// miniaturas son pequeñas y no deben desplazar la página que se está
	// leyendo ni las páginas precargadas.
	defaultThumbCacheBudget int64 = 24 << 20
	defaultThumbMaxSize           = 160
)

var errComicClosed = errors.New("el cómic está cerrado")

// Extensiones que image.Decode puede manejar con los decodificadores importados.
var comicImageExtensions = map[string]struct{}{
	".avif": {},
	".bmp":  {},
	".gif":  {},
	".jpe":  {},
	".jpeg": {},
	".jfif": {},
	".jpg":  {},
	".png":  {},
	".tif":  {},
	".tiff": {},
	".webp": {},
}

// Comic representa un cómic abierto y su índice de páginas.
//
// Soporta dos contenedores:
//   - RAR v4/v5 (extensión habitual .cbr).
//   - ZIP (extensión .cbz y también muchos .cbr mal nombrados en internet).
//
// En RAR los *rardecode.File conservan solo metadatos; en ZIP se conserva el
// lector central de archive/zip. En ambos casos cada Page() lee únicamente la
// entrada solicitada, sin descomprimir el cómic completo.
type Comic struct {
	Path  string
	Pages []string

	rarFiles map[string]*rardecode.File
	zipFile  *zip.ReadCloser
	zipFiles map[string]*zip.File

	cache  *pageCache
	thumbs *pageCache

	mu       sync.Mutex
	inflight map[string]*loadCall
	loadWG   sync.WaitGroup
	closed   bool
}

// loadCall permite que dos goroutines que piden la misma página compartan una
// única descompresión/decodificación (single-flight).
type loadCall struct {
	key    string
	done   chan struct{}
	img    *image.RGBA
	err    error
	refs   int
	cancel context.CancelFunc
}

type archiveKind int

const (
	archiveRAR archiveKind = iota
	archiveZIP
)

// OpenComic indexa un .cbr/.cbz sin descomprimir su contenido.
//
// El tipo de contenedor se detecta por los magic bytes, no por la extensión:
// es frecuente encontrar archivos llamados .cbr que en realidad son ZIP.
func OpenComic(comicPath string) (*Comic, error) {
	comicPath = strings.TrimSpace(comicPath)
	if comicPath == "" {
		return nil, errors.New("la ruta del cómic está vacía")
	}

	kind, err := detectArchiveKind(comicPath)
	if err != nil {
		return nil, err
	}
	if kind == archiveZIP {
		return openZIPComic(comicPath)
	}
	return openRARComic(comicPath)
}

func detectArchiveKind(comicPath string) (archiveKind, error) {
	f, err := os.Open(comicPath)
	if err != nil {
		return 0, fmt.Errorf("no se pudo abrir %q: %w", comicPath, err)
	}
	defer f.Close()

	var header [8]byte
	n, err := io.ReadFull(f, header[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("no se pudo leer %q: %w", comicPath, err)
	}

	data := header[:n]
	switch {
	case bytes.HasPrefix(data, []byte("Rar!\x1a\x07")):
		return archiveRAR, nil
	case bytes.HasPrefix(data, []byte("PK\x03\x04")), // ZIP normal
		bytes.HasPrefix(data, []byte("PK\x05\x06")), // ZIP vacío
		bytes.HasPrefix(data, []byte("PK\x07\x08")): // ZIP multi-volumen
		return archiveZIP, nil
	default:
		return 0, fmt.Errorf("%q no es un RAR ni un ZIP válido (¿CBR/CBZ correcto?)", comicPath)
	}
}

func openRARComic(comicPath string) (*Comic, error) {
	files, err := rardecode.List(comicPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el RAR %q: %w", comicPath, err)
	}

	c := newComic(comicPath)
	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		if f.IsDir || f.LinkType != rardecode.LinkTypeNone {
			continue
		}
		if isHiddenComicFile(f) || !isComicImage(f.Name) {
			continue
		}
		if !c.addPage(f.Name, seen) {
			continue
		}
		c.rarFiles[f.Name] = f
	}

	if err := c.finishIndex(); err != nil {
		return nil, err
	}
	return c, nil
}

func openZIPComic(comicPath string) (*Comic, error) {
	zr, err := zip.OpenReader(comicPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el ZIP %q: %w", comicPath, err)
	}

	c := newComic(comicPath)
	c.zipFile = zr
	c.zipFiles = make(map[string]*zip.File, len(zr.File))

	seen := make(map[string]struct{}, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || isHiddenComicPath(f.Name) || !isComicImage(f.Name) {
			continue
		}
		// archive/zip no descifra entradas con contraseña.
		if f.Flags&0x1 != 0 {
			continue
		}
		if !c.addPage(f.Name, seen) {
			continue
		}
		c.zipFiles[f.Name] = f
	}

	if err := c.finishIndex(); err != nil {
		_ = zr.Close()
		return nil, err
	}
	return c, nil
}

func newComic(comicPath string) *Comic {
	return &Comic{
		Path:     comicPath,
		rarFiles: make(map[string]*rardecode.File),
		zipFiles: make(map[string]*zip.File),
		cache:    newPageCache(defaultPageCacheBudget),
		thumbs:   newPageCache(defaultThumbCacheBudget),
		inflight: make(map[string]*loadCall),
	}
}

func (c *Comic) addPage(name string, seen map[string]struct{}) bool {
	if _, dup := seen[name]; dup {
		return false
	}
	seen[name] = struct{}{}
	c.Pages = append(c.Pages, name)
	return true
}

func (c *Comic) finishIndex() error {
	if len(c.Pages) == 0 {
		return fmt.Errorf("%q no contiene imágenes compatibles", c.Path)
	}

	// Orden alfabético natural: page2.jpg va antes que page10.jpg, que es lo
	// que espera cualquier lector de cómics.
	sort.SliceStable(c.Pages, func(i, j int) bool {
		return naturalLess(c.Pages[i], c.Pages[j])
	})
	return nil
}

// CancelLoads cancela todas las lecturas en curso. La UI lo usa al cambiar de
// página para que un prefetch o una carga antigua deje de consumir CPU/disco.
func (c *Comic) CancelLoads() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, call := range c.inflight {
		call.cancel()
		// Se retira del mapa para que una carga nueva del mismo nombre no se
		// enganche al trabajo cancelado. El worker compara punteros antes de
		// borrar para no eliminar una llamada más reciente.
		delete(c.inflight, key)
	}
}

// Close cancela las lecturas, espera a que terminen y solo después cierra el
// descriptor del ZIP. Así un prefetch en vuelo nunca lee de un archivo cerrado.
func (c *Comic) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	for _, call := range c.inflight {
		call.cancel()
	}
	c.mu.Unlock()

	c.loadWG.Wait()

	if c.zipFile != nil {
		return c.zipFile.Close()
	}
	return nil
}

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
	if maxSize <= 0 {
		maxSize = defaultThumbMaxSize
	}
	if maxSize > 1024 {
		maxSize = 1024
	}

	cacheKey := fmt.Sprintf("%s@%d", imageName, maxSize)
	return c.loadImage(ctx, "thumb:"+cacheKey, cacheKey, c.thumbs, func(loadCtx context.Context) (*image.RGBA, error) {
		data, err := c.readEntry(loadCtx, imageName)
		if err != nil {
			return nil, err
		}
		src, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("no se pudo decodificar %q para la miniatura: %w", imageName, err)
		}
		return makeThumbnail(src, maxSize), nil
	})
}

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
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
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

// loadPage extrae y decodifica una entrada concreta.
func (c *Comic) loadPage(ctx context.Context, imageName string) (*image.RGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := c.readEntry(ctx, imageName)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("no se pudo decodificar %q: %w", imageName, err)
	}

	// Normalización obligatoria: GIF animado, WebP, AVIF y algunos PNG
	// devuelven image.Paletted/NRGBA/YCbCr. Fyne y SubImage necesitan RGBA.
	return toRGBA(src), nil
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
		if header.IsDir || isHiddenComicPath(header.Name) || !isComicImage(header.Name) {
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

// isComicImage filtra por extensión soportada por los decodificadores.
func isComicImage(name string) bool {
	ext := strings.ToLower(path.Ext(strings.ReplaceAll(name, "\\", "/")))
	_, ok := comicImageExtensions[ext]
	return ok
}

// isHiddenComicPath ignora archivos ocultos, __MACOSX y metadatos de sistema.
func isHiddenComicPath(name string) bool {
	name = strings.ReplaceAll(name, "\\", "/")
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if strings.HasPrefix(part, ".") || part == "__MACOSX" {
			return true
		}
	}
	return false
}

// isHiddenComicFile combina la convención Unix (.nombre) con el atributo
// FILE_ATTRIBUTE_HIDDEN que WinRAR puede conservar en archivos de Windows.
func isHiddenComicFile(f *rardecode.File) bool {
	const fileAttributeHidden = 0x2
	if f.HostOS == rardecode.HostOSWindows && f.Attributes&fileAttributeHidden != 0 {
		return true
	}
	return isHiddenComicPath(f.Name)
}

// naturalLess compara nombres de forma alfabética pero numérica en los tramos
// de dígitos: "p2.jpg" < "p10.jpg" y "001.jpg" == "1.jpg" (desempata el nombre).
func naturalLess(a, b string) bool {
	if cmp := naturalCompare(a, b); cmp != 0 {
		return cmp < 0
	}
	return a < b
}

func naturalCompare(a, b string) int {
	ar := []rune(strings.ToLower(a))
	br := []rune(strings.ToLower(b))
	i, j := 0, 0

	for i < len(ar) && j < len(br) {
		if unicode.IsDigit(ar[i]) && unicode.IsDigit(br[j]) {
			si, sj := i, j
			for i < len(ar) && unicode.IsDigit(ar[i]) {
				i++
			}
			for j < len(br) && unicode.IsDigit(br[j]) {
				j++
			}

			an := strings.TrimLeft(string(ar[si:i]), "0")
			bn := strings.TrimLeft(string(br[sj:j]), "0")
			if len(an) != len(bn) {
				return sign(len(an) - len(bn))
			}
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
			continue
		}

		if ar[i] != br[j] {
			if ar[i] < br[j] {
				return -1
			}
			return 1
		}
		i++
		j++
	}

	switch {
	case len(ar)-i < len(br)-j:
		return -1
	case len(ar)-i > len(br)-j:
		return 1
	default:
		return 0
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

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
