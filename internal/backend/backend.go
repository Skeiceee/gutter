// Package backend contiene el motor de lectura de cómics de Gutter.
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
//
// El código está dividido por responsabilidad, todo dentro del mismo paquete:
//
//   - backend.go: núcleo (Comic, opciones, constantes, ciclo de vida y stats).
//   - open.go: detección del contenedor e indexado de RAR/ZIP.
//   - page.go: lectura, decodificación y recorte de páginas.
//   - load.go: caché LRU y single-flight compartido por páginas y miniaturas.
//   - prefetch.go: precarga secuencial de páginas.
//   - thumbnails.go: generación y persistencia de miniaturas.
//   - names.go: filtros de extensión/basura y orden natural.
//   - cache.go: implementación de la caché LRU en bytes.
//   - disk_cache.go: caché persistente de índices y páginas decodificadas.
//   - integrity.go: verificación de integridad con presupuesto de tiempo.
//   - options.go: opciones de apertura y modos de orden.
package backend

import (
	"archive/zip"
	"context"
	"errors"
	"image"
	"runtime"
	"sync"

	"github.com/nwaples/rardecode/v2"
)

const (
	// defaultPageCacheBudget es el presupuesto de RAM para páginas ya
	// decodificadas (RGBA). Con scans de ~12 MB caben cinco páginas: la actual,
	// dos por delante y una por detrás. Subirlo apenas acelera la lectura y
	// mantiene decenas de MiB ocupados de forma permanente.
	defaultPageCacheBudget int64 = 64 << 20 // 64 MiB

	// maxPageBytes protege contra entradas corruptas o maliciosas que declaren
	// un tamaño desmesurado. Una página de cómic rara vez supera los 256 MiB.
	maxPageBytes int64 = 256 << 20

	// maxPagePixels acota los píxeles que puede declarar una imagen. maxPageBytes
	// limita el tamaño comprimido, pero un PNG de 20 KB puede declarar
	// 100 000 × 100 000 píxeles: el decodificador reservaría ~40 GB de RGBA y
	// tumbaría el proceso. 40 MP (160 MB en RGBA) cubre con holgura un escaneo a
	// doble página o una tira vertical larga y deja margen frente a la caché.
	maxPagePixels int64 = 40_000_000

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

	opts         OpenOptions
	diskIdentity string

	rarFiles map[string]*rardecode.File
	zipFile  *zip.ReadCloser
	zipFiles map[string]*zip.File

	cache  *pageCache
	thumbs *pageCache

	// Problemas detectados por la comprobación rápida de integridad. La lectura
	// continúa: cada página afectada mostrará un marcador con reintento.
	problems []PageProblem

	// Los RAR se pueden indexar desde la caché en disco sin cargar sus
	// metadatos. rarFiles se rellena la primera vez que hace falta leer una
	// página; rarSolid permite elegir el recorrido secuencial.
	rarMu    sync.Mutex
	rarReady bool
	rarSolid bool

	mu       sync.Mutex
	inflight map[string]*loadCall
	loadWG   sync.WaitGroup
	diskWG   sync.WaitGroup
	// scanWG cuenta los recorridos secuenciales de un RAR sólido (prefetch y
	// miniaturas). No pasan por loadWG porque abren su propio lector, pero Close
	// debe esperarlos igualmente antes de dar el cómic por cerrado.
	scanWG sync.WaitGroup

	// lifeCtx se cancela en Close. Los recorridos secuenciales lo combinan con
	// el contexto del llamador para que un cierre corte la lectura aunque quien
	// la pidió no haya cancelado nada.
	lifeCtx    context.Context
	lifeCancel context.CancelFunc

	closed bool
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
	if c.lifeCancel != nil {
		c.lifeCancel()
	}
	c.mu.Unlock()

	c.loadWG.Wait()
	c.scanWG.Wait()
	c.diskWG.Wait()

	if c.zipFile != nil {
		return c.zipFile.Close()
	}
	return nil
}

// beginScan registra un recorrido secuencial en curso. Devuelve false si el
// cómic ya está cerrado. El Add se hace con c.mu tomado, igual que el cierre,
// para que Close nunca empiece a esperar antes de que el WaitGroup lo cuente.
func (c *Comic) beginScan() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	c.scanWG.Add(1)
	return true
}

// scanContext vincula el contexto del llamador con la vida del cómic: si Close
// cancela lifeCtx, el recorrido secuencial se detiene aunque quien lo pidió no
// haya cancelado su contexto.
func (c *Comic) scanContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	if c.lifeCtx == nil {
		return ctx, cancel
	}
	stop := context.AfterFunc(c.lifeCtx, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}

// MemoryStats describe el uso actual de las cachés en RAM.
type MemoryStats struct {
	PageCount  int
	PageBytes  int64
	PageLimit  int64
	ThumbCount int
	ThumbBytes int64
	ThumbLimit int64
}

// MemoryStats devuelve una foto de las cachés en RAM.
func (c *Comic) MemoryStats() MemoryStats {
	if c == nil {
		return MemoryStats{}
	}
	pageCount, pageBytes, pageLimit := c.cache.stats()
	thumbCount, thumbBytes, thumbLimit := c.thumbs.stats()
	return MemoryStats{
		PageCount:  pageCount,
		PageBytes:  pageBytes,
		PageLimit:  pageLimit,
		ThumbCount: thumbCount,
		ThumbBytes: thumbBytes,
		ThumbLimit: thumbLimit,
	}
}

// ClearMemoryCaches vacía las páginas y miniaturas en RAM. La página que se
// está mostrando sigue viva porque el canvas mantiene su propia referencia.
//
// Se fuerza una recolección: sin ella el siguiente pico se calcula sobre el
// heap que acaba de quedar basura y Go no vuelve a recolectar hasta duplicarlo.
// En la rejilla eso significaba reservar ~120 MB en vez de ~30 MB. La pausa es
// de milisegundos y solo ocurre al abrir una vista que no usa las páginas.
func (c *Comic) ClearMemoryCaches() {
	if c == nil {
		return
	}
	c.cache.clear()
	c.thumbs.clear()
	runtime.GC()
}

// IsSolid indica si el cómic es un RAR sólido. La rejilla lo usa para pedir
// todas las miniaturas en un solo recorrido secuencial en vez de uno por fila.
// El dato ya viene del índice en disco o del listado inicial, así que no hace
// falta releer las cabeceras del contenedor.
func (c *Comic) IsSolid() bool {
	if c == nil || c.zipFile != nil {
		return false
	}
	return c.rarSolid
}

// Problems devuelve una copia de las entradas problemáticas detectadas al abrir.
func (c *Comic) Problems() []PageProblem {
	if c == nil || len(c.problems) == 0 {
		return nil
	}
	out := make([]PageProblem, len(c.problems))
	copy(out, c.problems)
	return out
}
