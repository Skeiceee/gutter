package backend

// disk_cache.go implementa la caché persistente de Gutter.
//
// Hay dos tipos de archivos:
//   - Índice del contenedor (.json): nombres de página, problemas detectados y
//     datos del archivo original (tamaño + fecha de modificación). Reabrir un
//     RAR grande evita volver a recorrer sus cabeceras.
//   - Páginas decodificadas (.gtp): RGBA sin comprimir con una cabecera binaria
//     pequeña. Se prioriza la velocidad de lectura: descomprimir en cada
//     apertura costaba más que decodificar el JPEG original. El formato solo lo
//     lee Gutter y se puede borrar en cualquier momento.
//
// La caché es opcional. Si el directorio no se puede crear, todas las
// operaciones se convierten en no-op y la aplicación sigue funcionando igual.

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultDiskCacheLimit es el presupuesto inicial de páginas en disco.
	DefaultDiskCacheLimit int64 = 512 << 20 // 512 MiB

	diskIndexVersion = 1
	diskPageVersion  = uint32(2)
	diskPageMagic    = "GTP1"

	// Las miniaturas comparten el cómic y la fecha de la identidad, pero viven
	// en un directorio y un presupuesto aparte para que no expulsen páginas.
	maxThumbDiskLimit int64 = 128 << 20
	minThumbDiskLimit int64 = 16 << 20
)

// indexCacheEntry es el JSON que se guarda por cómic y configuración.
type indexCacheEntry struct {
	Version     int           `json:"version"`
	Path        string        `json:"path"`
	Size        int64         `json:"size"`
	ModTime     int64         `json:"mtime"`
	Sort        SortMode      `json:"sort"`
	IncludeJunk bool          `json:"include_junk"`
	Checked     bool          `json:"checked"`
	Solid       bool          `json:"solid,omitempty"`
	Pages       []string      `json:"pages"`
	Problems    []PageProblem `json:"problems,omitempty"`
}

// DiskCacheStats es una foto instantánea para «Acerca de».
type DiskCacheStats struct {
	Indexes    int
	IndexBytes int64
	Pages      int
	PageBytes  int64
	PageLimit  int64
	Thumbs     int
	ThumbBytes int64
	ThumbLimit int64
}

// DiskCache administra los archivos de caché. Es seguro para uso concurrente.
type DiskCache struct {
	mu sync.Mutex

	dir       string
	indexDir  string
	pagesDir  string
	thumbsDir string

	limit      int64
	thumbLimit int64
	generation uint64
	pageBytes  int64
	pageCount  int
	thumbBytes int64
	thumbCount int
	indexBytes int64
	indexCount int
}

// NewDiskCache prepara el directorio y cuenta lo que ya había. Un límite de 0
// desactiva la caché de páginas, pero mantiene la del índice.
func NewDiskCache(dir string, limit int64) *DiskCache {
	if limit < 0 {
		limit = 0
	}
	d := &DiskCache{
		dir:        dir,
		indexDir:   filepath.Join(dir, "index"),
		pagesDir:   filepath.Join(dir, "pages"),
		thumbsDir:  filepath.Join(dir, "thumbs"),
		limit:      limit,
		thumbLimit: thumbDiskBudget(limit),
	}
	if dir == "" {
		d.dir = ""
		return d
	}
	if err := os.MkdirAll(d.indexDir, 0o755); err != nil {
		d.dir = ""
		return d
	}
	if err := os.MkdirAll(d.pagesDir, 0o755); err != nil {
		d.dir = ""
		return d
	}
	if err := os.MkdirAll(d.thumbsDir, 0o755); err != nil {
		d.dir = ""
		return d
	}
	d.scan()
	return d
}

// thumbDiskBudget deriva un presupuesto pequeño para las miniaturas. No debe
// competir con las páginas completas, que son las que sostienen la lectura.
func thumbDiskBudget(pageLimit int64) int64 {
	if pageLimit <= 0 {
		return 0
	}
	budget := pageLimit / 4
	if budget < minThumbDiskLimit {
		budget = minThumbDiskLimit
	}
	if budget > maxThumbDiskLimit {
		budget = maxThumbDiskLimit
	}
	return budget
}

// Enabled indica si la caché tiene un directorio utilizable.
func (d *DiskCache) Enabled() bool {
	return d != nil && d.dir != ""
}

// SetLimit cambia el presupuesto de páginas y aplica la evicción de inmediato.
func (d *DiskCache) SetLimit(limit int64) {
	if d == nil {
		return
	}
	if limit < 0 {
		limit = 0
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.limit = limit
	d.thumbLimit = thumbDiskBudget(limit)
	d.generation++
	if limit == 0 {
		d.removeAllPagesLocked()
		d.removeAllThumbsLocked()
		return
	}
	d.enforceLimitLocked()
	d.enforceThumbLimitLocked()
}

// Stats devuelve los contadores actuales sin tocar el disco.
func (d *DiskCache) Stats() DiskCacheStats {
	if d == nil {
		return DiskCacheStats{}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return DiskCacheStats{
		Indexes:    d.indexCount,
		IndexBytes: d.indexBytes,
		Pages:      d.pageCount,
		PageBytes:  d.pageBytes,
		PageLimit:  d.limit,
		Thumbs:     d.thumbCount,
		ThumbBytes: d.thumbBytes,
		ThumbLimit: d.thumbLimit,
	}
}

// Clear borra todos los archivos de la caché y reinicia los contadores.
func (d *DiskCache) Clear() error {
	if d == nil || !d.Enabled() {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := os.RemoveAll(d.dir); err != nil {
		return err
	}
	if err := os.MkdirAll(d.indexDir, 0o755); err != nil {
		d.dir = ""
		return err
	}
	if err := os.MkdirAll(d.pagesDir, 0o755); err != nil {
		d.dir = ""
		return err
	}
	if err := os.MkdirAll(d.thumbsDir, 0o755); err != nil {
		d.dir = ""
		return err
	}
	d.pageBytes, d.pageCount = 0, 0
	d.thumbBytes, d.thumbCount = 0, 0
	d.indexBytes, d.indexCount = 0, 0
	d.generation++
	return nil
}

// LoadIndex devuelve el índice guardado si sigue siendo válido para este
// archivo y esta configuración.
func (d *DiskCache) LoadIndex(identity string, info os.FileInfo, opts OpenOptions) (*indexCacheEntry, bool) {
	if !d.Enabled() || identity == "" || info == nil {
		return nil, false
	}
	opts = normalizeOpenOptions(opts)
	name := d.indexName(identity, opts)
	data, err := os.ReadFile(filepath.Join(d.indexDir, name))
	if err != nil {
		return nil, false
	}

	var entry indexCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		_ = os.Remove(filepath.Join(d.indexDir, name))
		return nil, false
	}
	if entry.Version != diskIndexVersion ||
		entry.Size != info.Size() ||
		entry.ModTime != info.ModTime().UnixNano() ||
		entry.Sort != opts.Sort ||
		entry.IncludeJunk != opts.IncludeJunk ||
		entry.Checked != opts.IntegrityCheck ||
		len(entry.Pages) == 0 {
		_ = os.Remove(filepath.Join(d.indexDir, name))
		return nil, false
	}
	return &entry, true
}

// SaveIndex guarda el índice del contenedor. Es best-effort: un error de disco
// no debe impedir leer el cómic.
func (d *DiskCache) SaveIndex(identity string, info os.FileInfo, opts OpenOptions, solid bool, pages []string, problems []PageProblem) {
	if !d.Enabled() || identity == "" || info == nil || len(pages) == 0 {
		return
	}
	opts = normalizeOpenOptions(opts)

	entry := indexCacheEntry{
		Version:     diskIndexVersion,
		Path:        info.Name(),
		Size:        info.Size(),
		ModTime:     info.ModTime().UnixNano(),
		Sort:        opts.Sort,
		IncludeJunk: opts.IncludeJunk,
		Checked:     opts.IntegrityCheck,
		Solid:       solid,
		Pages:       append([]string(nil), pages...),
		Problems:    append([]PageProblem(nil), problems...),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dir == "" {
		return
	}
	dst := filepath.Join(d.indexDir, d.indexName(identity, opts))
	old := fileSize(dst)
	if size, err := writeFileAtomic(dst, data); err == nil {
		if old > 0 {
			d.indexBytes -= old
			d.indexCount--
		}
		d.indexBytes += size
		d.indexCount++
		if d.indexCount < 0 {
			d.indexCount = 0
		}
	}
}

// LoadPage intenta recuperar una página decodificada. Devuelve false ante
// cualquier problema: el llamador simplemente descomprime la entrada original.
func (d *DiskCache) LoadPage(identity, name string) (*image.RGBA, bool) {
	if !d.Enabled() || identity == "" || name == "" {
		return nil, false
	}
	dst := filepath.Join(d.pagesDir, d.pageName(identity, name))
	f, err := os.Open(dst)
	if err != nil {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(f, maxPageBytes+(1<<20)))
	_ = f.Close()
	if err != nil {
		return nil, false
	}

	img, err := decodeDiskPage(data)
	if err != nil {
		_ = os.Remove(dst)
		return nil, false
	}

	// Marca de uso para que la evicción sea aproximadamente LRU.
	now := time.Now()
	_ = os.Chtimes(dst, now, now)
	return img, true
}

// SavePage guarda una página en segundo plano. El llamador no espera: la
// codificación y la escritura quedan fuera del hilo de interfaz.
func (d *DiskCache) SavePage(identity, name string, img *image.RGBA) {
	if !d.Enabled() || identity == "" || name == "" || img == nil {
		return
	}

	d.mu.Lock()
	limit := d.limit
	generation := d.generation
	d.mu.Unlock()
	if limit <= 0 {
		return
	}

	payload, err := encodeDiskPage(img)
	if err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dir == "" || d.limit <= 0 || d.generation != generation {
		return
	}

	dst := filepath.Join(d.pagesDir, d.pageName(identity, name))
	old := fileSize(dst)
	size, err := writeFileAtomic(dst, payload)
	if err != nil {
		return
	}
	if old > 0 {
		d.pageBytes -= old
		d.pageCount--
	}
	d.pageBytes += size
	d.pageCount++
	if d.pageCount < 0 {
		d.pageCount = 0
	}
	d.enforceLimitLocked()
}

// LoadThumbnail recupera una miniatura persistida. El tamaño máximo forma
// parte de la clave para que cambiar el tamaño de la rejilla no reutilice una
// imagen con la resolución equivocada.
func (d *DiskCache) LoadThumbnail(identity, name string, maxSize int) (*image.RGBA, bool) {
	if !d.Enabled() || identity == "" || name == "" {
		return nil, false
	}
	maxSize = normalizeThumbSize(maxSize)
	dst := filepath.Join(d.thumbsDir, d.thumbName(identity, name, maxSize))
	f, err := os.Open(dst)
	if err != nil {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(f, maxPageBytes+(1<<20)))
	_ = f.Close()
	if err != nil {
		return nil, false
	}

	img, err := decodeDiskPage(data)
	if err != nil {
		_ = os.Remove(dst)
		return nil, false
	}

	now := time.Now()
	_ = os.Chtimes(dst, now, now)
	return img, true
}

// SaveThumbnail guarda una miniatura con el mismo formato binario que las
// páginas. Se llama en segundo plano desde el backend.
func (d *DiskCache) SaveThumbnail(identity, name string, maxSize int, img *image.RGBA) {
	if !d.Enabled() || identity == "" || name == "" || img == nil {
		return
	}
	maxSize = normalizeThumbSize(maxSize)

	d.mu.Lock()
	limit := d.thumbLimit
	generation := d.generation
	d.mu.Unlock()
	if limit <= 0 {
		return
	}

	payload, err := encodeDiskPage(img)
	if err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dir == "" || d.thumbLimit <= 0 || d.generation != generation {
		return
	}

	dst := filepath.Join(d.thumbsDir, d.thumbName(identity, name, maxSize))
	old := fileSize(dst)
	size, err := writeFileAtomic(dst, payload)
	if err != nil {
		return
	}
	if old > 0 {
		d.thumbBytes -= old
		d.thumbCount--
	}
	d.thumbBytes += size
	d.thumbCount++
	if d.thumbCount < 0 {
		d.thumbCount = 0
	}
	d.enforceThumbLimitLocked()
}

// indexName usa una clave estable por cómic y configuración.
func (d *DiskCache) indexName(identity string, opts OpenOptions) string {
	return diskHash(
		identity,
		string(opts.Sort),
		fmt.Sprintf("%t", opts.IncludeJunk),
		fmt.Sprintf("%t", opts.IntegrityCheck),
	) + ".json"
}

func (d *DiskCache) pageName(identity, name string) string {
	return diskHash(identity, name) + ".gtp"
}

func (d *DiskCache) thumbName(identity, name string, maxSize int) string {
	return diskHash(identity, name, strconv.Itoa(maxSize)) + ".gth"
}

func (d *DiskCache) scan() {
	entries, err := os.ReadDir(d.indexDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			d.indexCount++
			d.indexBytes += info.Size()
		}
	}

	entries, err = os.ReadDir(d.pagesDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gtp") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			d.pageCount++
			d.pageBytes += info.Size()
		}
	}

	entries, err = os.ReadDir(d.thumbsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gth") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			d.thumbCount++
			d.thumbBytes += info.Size()
		}
	}
	d.enforceLimitLocked()
	d.enforceThumbLimitLocked()
}

// enforceLimitLocked elimina las páginas menos usadas hasta cumplir el límite.
// Se llama con d.mu tomado.
func (d *DiskCache) enforceLimitLocked() {
	d.enforceDirLimitLocked(d.pagesDir, ".gtp", &d.pageBytes, &d.pageCount, d.limit)
}

// enforceThumbLimitLocked hace lo mismo con el presupuesto de miniaturas.
func (d *DiskCache) enforceThumbLimitLocked() {
	d.enforceDirLimitLocked(d.thumbsDir, ".gth", &d.thumbBytes, &d.thumbCount, d.thumbLimit)
}

// enforceDirLimitLocked comparte la evicción aproximada por fecha de uso entre
// páginas y miniaturas.
func (d *DiskCache) enforceDirLimitLocked(dir, suffix string, bytes *int64, count *int, limit int64) {
	if d.dir == "" || limit <= 0 || *bytes <= limit {
		return
	}

	type diskFile struct {
		name    string
		size    int64
		modTime time.Time
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	files := make([]diskFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, diskFile{name: entry.Name(), size: info.Size(), modTime: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })

	for _, file := range files {
		if *bytes <= limit {
			break
		}
		if err := os.Remove(filepath.Join(dir, file.name)); err != nil {
			break
		}
		*bytes -= file.size
		*count--
	}
	if *count < 0 {
		*count = 0
	}
	if *bytes < 0 {
		*bytes = 0
	}
}

func (d *DiskCache) removeAllPagesLocked() {
	_ = os.RemoveAll(d.pagesDir)
	_ = os.MkdirAll(d.pagesDir, 0o755)
	d.pageBytes, d.pageCount = 0, 0
}

func (d *DiskCache) removeAllThumbsLocked() {
	_ = os.RemoveAll(d.thumbsDir)
	_ = os.MkdirAll(d.thumbsDir, 0o755)
	d.thumbBytes, d.thumbCount = 0, 0
}

// archiveIdentity combina ruta, tamaño y fecha de modificación. Si el archivo
// cambia, la identidad cambia y las entradas antiguas dejan de ser válidas.
func archiveIdentity(comicPath string, info os.FileInfo) string {
	if info == nil {
		return ""
	}
	key := fmt.Sprintf("%s|%d|%d",
		strings.ToLower(filepath.Clean(comicPath)),
		info.Size(),
		info.ModTime().UnixNano(),
	)
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func diskHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeFileAtomic(dst string, data []byte) (int64, error) {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return 0, err
	}
	// Windows no permite Rename sobre un archivo existente.
	_ = os.Remove(dst)
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return 0, err
	}
	return int64(len(data)), nil
}

func fileSize(name string) int64 {
	info, err := os.Stat(name)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

// encodeDiskPage produce: "GTP1" + versión + ancho + alto + RGBA crudo.
// El tamaño en disco es mayor, pero cargar una página es una copia contigua:
// comprimir y descomprimir en cada apertura resultaba más lento que decodificar
// el JPEG original.
func encodeDiskPage(img *image.RGBA) ([]byte, error) {
	if img == nil {
		return nil, errors.New("imagen vacía")
	}
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if width <= 0 || height <= 0 {
		return nil, errors.New("imagen sin dimensiones")
	}
	rawSize := int64(width) * int64(height) * 4
	if rawSize > maxPageBytes {
		return nil, fmt.Errorf("la imagen ocupa %d MiB y supera el límite", rawSize>>20)
	}

	data := make([]byte, 16+rawSize)
	copy(data[0:4], diskPageMagic)
	binary.LittleEndian.PutUint32(data[4:8], diskPageVersion)
	binary.LittleEndian.PutUint32(data[8:12], uint32(width))
	binary.LittleEndian.PutUint32(data[12:16], uint32(height))

	rowBytes := width * 4
	dst := data[16:]
	if img.Stride == rowBytes {
		start := img.PixOffset(b.Min.X, b.Min.Y)
		copy(dst, img.Pix[start:start+rowBytes*height])
	} else {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			start := img.PixOffset(b.Min.X, y)
			offset := (y - b.Min.Y) * rowBytes
			copy(dst[offset:offset+rowBytes], img.Pix[start:start+rowBytes])
		}
	}
	return data, nil
}

func decodeDiskPage(data []byte) (*image.RGBA, error) {
	if len(data) < 16 || string(data[:4]) != diskPageMagic {
		return nil, errors.New("cabecera de página en disco inválida")
	}
	if version := binary.LittleEndian.Uint32(data[4:8]); version != diskPageVersion {
		return nil, fmt.Errorf("versión de página en disco no soportada: %d", version)
	}
	width := int(binary.LittleEndian.Uint32(data[8:12]))
	height := int(binary.LittleEndian.Uint32(data[12:16]))
	if width <= 0 || height <= 0 {
		return nil, errors.New("dimensiones de página en disco inválidas")
	}
	rawSize := int64(width) * int64(height) * 4
	if rawSize > maxPageBytes {
		return nil, errors.New("la página en disco supera el límite de tamaño")
	}
	if int64(len(data)-16) != rawSize {
		return nil, errors.New("los píxeles de la página en disco están incompletos")
	}

	pix := make([]byte, rawSize)
	copy(pix, data[16:])
	return &image.RGBA{
		Pix:    pix,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}, nil
}
