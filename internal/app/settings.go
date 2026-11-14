package app

// settings.go reúne las preferencias de rendimiento y los diálogos que las
// modifican: opciones de lectura, aviso de integridad y estadísticas de caché.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/backend"
	"gutter/internal/ocr"
)

const (
	prefSortMode          = "reader.sort"
	prefFilterJunk        = "reader.filter_junk"
	prefIntegrityCheck    = "reader.integrity_check"
	prefGridThumbSize     = "reader.grid_thumb_size"
	prefDiskCacheLimitMB  = "cache.disk.limit_mb"
	prefRememberWindow    = "ui.remember_window"
	prefPageBadge         = "ui.page_badge"
	prefCheckUpdates      = "update.check_on_start"
	defaultDiskCacheLimit = 512 // MiB
	defaultGridThumbSize  = 160
)

var (
	diskLimitLabels = []string{"Desactivada", "256 MB", "512 MB", "1 GB", "2 GB", "4 GB"}
	diskLimitValues = []int{0, 256, 512, 1024, 2048, 4096}

	gridThumbLabels = []string{"Pequeñas (120 px)", "Medianas (160 px)", "Grandes (200 px)", "Muy grandes (240 px)"}
	gridThumbValues = []int{120, 160, 200, 240}
)

// newAppDiskCache crea la caché persistente dentro del directorio de caché de
// Fyne. Si no hay un directorio utilizable, devuelve una caché desactivada.
func newAppDiskCache(a fyne.App, prefs fyne.Preferences) *backend.DiskCache {
	limitMB := defaultDiskCacheLimit
	if prefs != nil {
		limitMB = prefs.IntWithFallback(prefDiskCacheLimitMB, defaultDiskCacheLimit)
	}
	limit := int64(limitMB) << 20

	dir := ""
	if a != nil {
		if cache := a.Cache(); cache != nil {
			if uri := cache.RootURI(); uri != nil {
				dir = filepath.Join(uri.Path(), "reader")
			}
		}
	}
	if dir == "" {
		if base, err := os.UserCacheDir(); err == nil && base != "" {
			dir = filepath.Join(base, "Gutter")
		}
	}
	return backend.NewDiskCache(dir, limit)
}

func (g *gutterApp) sortMode() backend.SortMode {
	if g.prefs != nil && g.prefs.String(prefSortMode) == string(backend.SortLexicographic) {
		return backend.SortLexicographic
	}
	return backend.SortNatural
}

func (g *gutterApp) filterJunk() bool {
	if g.prefs == nil {
		return true
	}
	return g.prefs.BoolWithFallback(prefFilterJunk, true)
}

func (g *gutterApp) integrityCheck() bool {
	if g.prefs == nil {
		return true
	}
	return g.prefs.BoolWithFallback(prefIntegrityCheck, true)
}

func (g *gutterApp) gridThumbSizePref() int {
	if g.prefs == nil {
		return defaultGridThumbSize
	}
	size := g.prefs.IntWithFallback(prefGridThumbSize, defaultGridThumbSize)
	for _, allowed := range gridThumbValues {
		if size == allowed {
			return size
		}
	}
	return defaultGridThumbSize
}

func gridThumbLabel(size int) string {
	for i, value := range gridThumbValues {
		if value == size {
			return gridThumbLabels[i]
		}
	}
	return gridThumbLabels[1]
}

func parseGridThumbSize(label string) int {
	for i, text := range gridThumbLabels {
		if text == label {
			return gridThumbValues[i]
		}
	}
	return defaultGridThumbSize
}

func (g *gutterApp) openOptions() backend.OpenOptions {
	return backend.OpenOptions{
		Sort:           g.sortMode(),
		IncludeJunk:    !g.filterJunk(),
		DiskCache:      g.diskCache,
		IntegrityCheck: g.integrityCheck(),
	}
}

func (g *gutterApp) diskLimitMB() int {
	if g.prefs == nil {
		return defaultDiskCacheLimit
	}
	return g.prefs.IntWithFallback(prefDiskCacheLimitMB, defaultDiskCacheLimit)
}

// showReadingOptions abre las preferencias directamente en la pestaña de
// lectura. Se conserva como punto de entrada para el menú y las pruebas; la
// implementación vive en settings_ui.go.
func (g *gutterApp) showReadingOptions() {
	g.showSettings(settingsTabReading)
}

// rememberWindowSize indica si el tamaño de la ventana debe persistirse entre
// sesiones.
func (g *gutterApp) rememberWindowSize() bool {
	if g.prefs == nil {
		return true
	}
	return g.prefs.BoolWithFallback(prefRememberWindow, true)
}

// showPageBadgePref controla el indicador «Página X / N» del modo limpio.
func (g *gutterApp) showPageBadgePref() bool {
	if g.prefs == nil {
		return true
	}
	return g.prefs.BoolWithFallback(prefPageBadge, true)
}

// checkUpdatesOnStart indica si se debe buscar una versión nueva al arrancar.
func (g *gutterApp) checkUpdatesOnStart() bool {
	if g.prefs == nil {
		return false
	}
	return g.prefs.BoolWithFallback(prefCheckUpdates, false)
}

// reopenThumbnailGrid conserva la página seleccionada al cambiar el tamaño de
// las miniaturas desde las opciones de lectura.
func (g *gutterApp) reopenThumbnailGrid() {
	if !g.gridOpen {
		return
	}
	index := g.gridSelection
	g.hideThumbnailGrid()
	g.showThumbnailGrid()
	if g.gridOpen {
		g.setGridSelection(index, true)
	}
}

// reloadPreservingPage reabre el cómic con los nuevos ajustes. El orden puede
// cambiar, así que se conserva el nombre de la página, no su índice.
func (g *gutterApp) reloadPreservingPage() {
	if g.reader.comic == nil {
		return
	}
	comicPath := g.reader.comic.Path
	pageName := ""
	if g.reader.pageIndex >= 0 && g.reader.pageIndex < len(g.reader.comic.Pages) {
		pageName = g.reader.comic.Pages[g.reader.pageIndex]
	}
	g.loadComic(comicPath, pageName)
}

// showIntegrityWarning avisa de las entradas corruptas sin impedir la lectura.
func (g *gutterApp) showIntegrityWarning(problems []backend.PageProblem) {
	if len(problems) == 0 {
		return
	}

	var body strings.Builder
	fmt.Fprintf(&body, "Se detectaron %d entradas con problemas.\n\n", len(problems))
	for i, problem := range problems {
		if i >= 8 {
			fmt.Fprintf(&body, "… y %d más.\n", len(problems)-8)
			break
		}
		fmt.Fprintf(&body, "• %s\n  %s\n", problem.Name, problem.Detail)
	}
	body.WriteString("\nPuedes seguir leyendo: las páginas afectadas mostrarán un marcador con Reintentar.")

	label := widget.NewLabel(body.String())
	label.Wrapping = fyne.TextWrapWord

	scroll := container.NewVScroll(label)
	scroll.SetMinSize(fyne.NewSize(460, 200))

	d := dialog.NewCustom("Aviso de integridad", "Continuar", scroll, g.window)
	d.Resize(fyne.NewSize(560, 360))
	g.showDialog(d)
}

func (g *gutterApp) clearCaches() {
	if g.reader.comic != nil {
		g.reader.comic.ClearMemoryCaches()
	}
	g.translationCache = make(map[string]*ocr.PageTranslation)
	if g.diskCache != nil {
		_ = g.diskCache.Clear()
	}
}

// cacheSummary genera el texto del indicador de «Acerca de».
func (g *gutterApp) cacheSummary() string {
	var body strings.Builder
	if g.reader.comic != nil {
		stats := g.reader.comic.MemoryStats()
		fmt.Fprintf(&body, "RAM (páginas): %d · %s de %s\n",
			stats.PageCount, formatBytes(stats.PageBytes), formatBytes(stats.PageLimit))
		fmt.Fprintf(&body, "RAM (miniaturas): %d · %s de %s\n",
			stats.ThumbCount, formatBytes(stats.ThumbBytes), formatBytes(stats.ThumbLimit))
	} else {
		body.WriteString("RAM: sin cómic abierto\n")
	}

	if g.diskCache != nil && g.diskCache.Enabled() {
		stats := g.diskCache.Stats()
		limit := "sin límite"
		switch {
		case stats.PageLimit == 0:
			limit = "desactivada"
		case stats.PageLimit > 0:
			limit = formatBytes(stats.PageLimit)
		}
		fmt.Fprintf(&body, "Disco (páginas): %d · %s de %s\n",
			stats.Pages, formatBytes(stats.PageBytes), limit)
		fmt.Fprintf(&body, "Disco (miniaturas): %d · %s de %s\n",
			stats.Thumbs, formatBytes(stats.ThumbBytes), formatBytes(stats.ThumbLimit))
		fmt.Fprintf(&body, "Disco (índices): %d · %s",
			stats.Indexes, formatBytes(stats.IndexBytes))
	} else {
		body.WriteString("Disco: caché no disponible")
	}
	return body.String()
}

func diskLimitLabel(mb int) string {
	for i, value := range diskLimitValues {
		if value == mb {
			return diskLimitLabels[i]
		}
	}
	return diskLimitLabels[2]
}

func parseDiskLimit(label string) int {
	for i, text := range diskLimitLabels {
		if text == label {
			return diskLimitValues[i]
		}
	}
	return defaultDiskCacheLimit
}

func formatBytes(bytes int64) string {
	if bytes < 0 {
		bytes = 0
	}
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(bytes)
	index := -1
	for value >= 1024 && index < len(units)-1 {
		value /= 1024
		index++
	}
	return fmt.Sprintf("%.1f %s", value, units[index])
}
