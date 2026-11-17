package backend

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/nwaples/rardecode/v2"
)

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
	return OpenComicWithOptions(comicPath, OpenOptions{
		Sort:        SortNatural,
		IncludeJunk: false,
	})
}

// OpenComicWithOptions permite elegir orden, filtro de basura, caché en disco y
// verificación de integridad. La aplicación usa esta variante; OpenComic
// conserva el comportamiento histórico para tests y llamadas simples.
func OpenComicWithOptions(comicPath string, opts OpenOptions) (*Comic, error) {
	comicPath = strings.TrimSpace(comicPath)
	if comicPath == "" {
		return nil, errors.New("la ruta del cómic está vacía")
	}
	opts = normalizeOpenOptions(opts)

	kind, err := detectArchiveKind(comicPath)
	if err != nil {
		return nil, err
	}
	if kind == archiveZIP {
		return openZIPComicWithOptions(comicPath, opts)
	}
	return openRARComicWithOptions(comicPath, opts)
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

func openRARComicWithOptions(comicPath string, opts OpenOptions) (*Comic, error) {
	info, _ := os.Stat(comicPath)
	c := newComicWithOptions(comicPath, opts)
	if info != nil {
		c.diskIdentity = archiveIdentity(comicPath, info)
	}

	// Índice persistente: si el archivo no ha cambiado, la lista de páginas y
	// los problemas detectados se recuperan sin recorrer las cabeceras del RAR.
	if opts.DiskCache != nil && c.diskIdentity != "" {
		if entry, ok := opts.DiskCache.LoadIndex(c.diskIdentity, info, opts); ok {
			c.Pages = append(c.Pages, entry.Pages...)
			c.problems = append(c.problems, entry.Problems...)
			c.rarSolid = entry.Solid
			return c, nil
		}
	}

	files, err := rardecode.List(comicPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el RAR %q: %w", comicPath, err)
	}

	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		if !c.acceptsRARFile(&f.FileHeader) {
			continue
		}
		if !c.addPage(f.Name, seen) {
			continue
		}
		c.rarFiles[f.Name] = f
		if f.Solid {
			c.rarSolid = true
		}
	}
	c.rarReady = true

	if err := c.finishIndex(); err != nil {
		return nil, err
	}
	if opts.IntegrityCheck {
		c.problems = c.quickIntegrityCheck()
	}
	if opts.DiskCache != nil && c.diskIdentity != "" {
		opts.DiskCache.SaveIndex(c.diskIdentity, info, opts, c.rarSolid, c.Pages, c.problems)
	}
	return c, nil
}

func openZIPComicWithOptions(comicPath string, opts OpenOptions) (*Comic, error) {
	zr, err := zip.OpenReader(comicPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el ZIP %q: %w", comicPath, err)
	}

	info, _ := os.Stat(comicPath)
	c := newComicWithOptions(comicPath, opts)
	if info != nil {
		c.diskIdentity = archiveIdentity(comicPath, info)
	}
	c.zipFile = zr
	c.zipFiles = make(map[string]*zip.File, len(zr.File))

	// Índice persistente: se reutiliza la lista de páginas ya ordenada y los
	// problemas detectados. El ZIP sigue abriéndose para acceder a los datos,
	// pero se evita repetir filtrado, ordenación y verificación.
	if opts.DiskCache != nil && c.diskIdentity != "" {
		if entry, ok := opts.DiskCache.LoadIndex(c.diskIdentity, info, opts); ok && c.restoreZIPIndex(entry) {
			return c, nil
		}
	}

	seen := make(map[string]struct{}, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !c.acceptsEntry(f.Name) {
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
	if opts.IntegrityCheck {
		c.problems = c.quickIntegrityCheck()
	}
	if opts.DiskCache != nil && c.diskIdentity != "" {
		opts.DiskCache.SaveIndex(c.diskIdentity, info, opts, false, c.Pages, c.problems)
	}
	return c, nil
}

// restoreZIPIndex rellena zipFiles a partir de las páginas guardadas. Si el
// archivo ya no contiene todas las entradas del índice, se descarta la caché
// para no mostrar páginas que fallarían al abrirse.
func (c *Comic) restoreZIPIndex(entry *indexCacheEntry) bool {
	if entry == nil || len(entry.Pages) == 0 {
		return false
	}
	wanted := make(map[string]struct{}, len(entry.Pages))
	for _, name := range entry.Pages {
		wanted[name] = struct{}{}
	}
	for _, f := range c.zipFile.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if _, ok := wanted[f.Name]; !ok {
			continue
		}
		if f.Flags&0x1 != 0 {
			continue
		}
		c.zipFiles[f.Name] = f
		delete(wanted, f.Name)
	}
	if len(wanted) != 0 {
		c.zipFiles = make(map[string]*zip.File, len(c.zipFile.File))
		return false
	}
	c.Pages = append([]string(nil), entry.Pages...)
	c.problems = append([]PageProblem(nil), entry.Problems...)
	return true
}

func newComicWithOptions(comicPath string, opts OpenOptions) *Comic {
	opts = normalizeOpenOptions(opts)
	lifeCtx, lifeCancel := context.WithCancel(context.Background())
	return &Comic{
		Path:       comicPath,
		opts:       opts,
		rarFiles:   make(map[string]*rardecode.File),
		zipFiles:   make(map[string]*zip.File),
		cache:      newPageCache(defaultPageCacheBudget),
		thumbs:     newPageCache(defaultThumbCacheBudget),
		inflight:   make(map[string]*loadCall),
		lifeCtx:    lifeCtx,
		lifeCancel: lifeCancel,
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

	if c.opts.Sort == SortLexicographic {
		sort.SliceStable(c.Pages, func(i, j int) bool {
			return lexicographicLess(c.Pages[i], c.Pages[j])
		})
		return nil
	}

	// Orden alfabético natural: page2.jpg va antes que page10.jpg, que es lo
	// que espera cualquier lector de cómics.
	sort.SliceStable(c.Pages, func(i, j int) bool {
		return naturalLess(c.Pages[i], c.Pages[j])
	})
	return nil
}
