package app

// state.go agrupa el estado puro de lectura y las operaciones que solo
// dependen de él: qué cómic está abierto, en qué página y mitad, cómo se
// pagina (occidental/manga, una página o doble) y el nivel de zoom.
//
// readerState no conoce widgets, contextos ni diálogos. Esa separación permite
// probar la navegación y la restauración de progreso sin construir la interfaz,
// y deja en gutterApp únicamente la orquestación (cargar, dibujar, avisar).

import (
	"fyne.io/fyne/v2"
	"gutter/internal/backend"
)

type readerState struct {
	comic      *backend.Comic
	pageIndex  int
	half       int  // mitad en orden de lectura: 0 = primera, 1 = segunda
	spread     bool // la página actual es horizontal
	singlePage bool
	manga      bool
	loading    bool
	webtoon    bool // modo tiras: páginas apiladas en vertical

	// zoomFit mide la página al ancho disponible; en modo manual, zoomScale es
	// la escala absoluta (1 = píxel de imagen = unidad Fyne).
	zoomFit   bool
	zoomScale float32
	viewport  fyne.Size

	// cleanMode oculta barras y cursor, y pasa a pantalla completa.
	cleanMode bool

	// Última página realmente dibujada. Permite restaurar la vista si el
	// usuario cancela una carga larga (por ejemplo, un salto en RAR sólido).
	shownComic  *backend.Comic
	shownPage   int
	shownHalf   int
	shownSpread bool
}

func newReaderState() readerState {
	return readerState{
		singlePage: true,
		zoomFit:    true,
		zoomScale:  1,
	}
}

// total devuelve el número de páginas del cómic abierto (0 si no hay ninguno).
func (s *readerState) total() int {
	if s.comic == nil {
		return 0
	}
	return len(s.comic.Pages)
}

// pageName devuelve el nombre de la entrada actual, o "" si el índice quedó
// fuera de rango.
func (s *readerState) pageName() string {
	if s.pageIndex < 0 || s.pageIndex >= s.total() {
		return ""
	}
	return s.comic.Pages[s.pageIndex]
}

// clampIndex mantiene página y mitad dentro de límites válidos.
func (s *readerState) clampIndex() {
	total := s.total()
	if total == 0 {
		s.pageIndex = 0
		s.half = 0
		return
	}
	if s.pageIndex < 0 {
		s.pageIndex = 0
	}
	if s.pageIndex >= total {
		s.pageIndex = total - 1
	}
	if s.half < 0 || s.half > 1 {
		s.half = 0
	}
}

// resetProgress deja el lector al principio y con el zoom automático. Se usa al
// abrir un cómic sin progreso guardado.
func (s *readerState) resetProgress() {
	s.webtoon = false
	s.pageIndex = 0
	s.half = 0
	s.spread = false
	s.zoomFit = true
	s.zoomScale = 1
}

// restoreFrom aplica una entrada de Recientes. El índice se acota al número de
// páginas actual porque el archivo puede haber cambiado.
func (s *readerState) restoreFrom(entry recentComic, total int) {
	s.pageIndex = clampPage(entry.Page, total)
	s.half = 0
	if entry.Half == 1 {
		s.half = 1
	}
	s.manga = entry.Manga
	s.singlePage = entry.SinglePage
	s.webtoon = entry.Webtoon
	s.zoomFit = entry.ZoomFit
	if entry.ZoomScale > 0 {
		s.zoomScale = clampZoom(entry.ZoomScale)
	}
}

// progressEntry construye la entrada que se guarda en Recientes.
func (s *readerState) progressEntry() (recentComic, bool) {
	if s.comic == nil || len(s.comic.Pages) == 0 {
		return recentComic{}, false
	}
	return recentComic{
		Path:       s.comic.Path,
		Page:       s.pageIndex,
		Half:       s.half,
		Total:      len(s.comic.Pages),
		Manga:      s.manga,
		SinglePage: s.singlePage,
		Webtoon:    s.webtoon,
		ZoomFit:    s.zoomFit,
		ZoomScale:  s.zoomScale,
	}, true
}

// normalizeHalf descarta la segunda mitad cuando la página no es doble o el
// modo una página está desactivado.
func (s *readerState) normalizeHalf() {
	if !s.singlePage || !s.spread {
		s.half = 0
	}
}

// next avanza una posición de lectura. Devuelve false si ya está al final.
// Durante una carga no conocemos si la página destino es un spread: se avanza
// de página directamente y showPage lo resuelve.
func (s *readerState) next() bool {
	total := s.total()
	if total == 0 {
		return false
	}
	if s.loading {
		if s.pageIndex >= total-1 {
			return false
		}
		s.pageIndex++
		s.half = 0
		s.spread = false // desconocido hasta que showPage lo resuelva
		return true
	}
	// Primero la segunda mitad de una doble página.
	if s.singlePage && s.spread && s.half == 0 {
		s.half = 1
		return true
	}
	if s.pageIndex >= total-1 {
		return false
	}
	s.pageIndex++
	s.half = 0
	s.spread = false // desconocido hasta que showPage lo resuelva
	return true
}

// previous retrocede una posición de lectura. Devuelve false si ya está al
// principio.
func (s *readerState) previous() bool {
	if s.total() == 0 {
		return false
	}
	if s.loading {
		if s.pageIndex <= 0 {
			return false
		}
		s.pageIndex--
		s.half = 0
		s.spread = false
		return true
	}
	if s.singlePage && s.spread && s.half == 1 {
		s.half = 0
		return true
	}
	if s.pageIndex <= 0 {
		return false
	}
	s.pageIndex--
	// Al retroceder, si la página anterior era doble, mostramos su última
	// mitad. showPage lo corrige a 0 si finalmente no es una doble página.
	s.half = 1
	s.spread = false
	return true
}

// goTo salta a una página concreta, acotada al rango disponible.
func (s *readerState) goTo(index int) {
	total := s.total()
	if total == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= total {
		index = total - 1
	}
	s.pageIndex = index
	s.half = 0
}

// isRightHalf indica qué mitad debe recortarse según la mitad actual y el
// sentido de lectura.
func (s *readerState) isRightHalf() bool {
	return isRightHalf(s.half, s.manga)
}

// canGoBack y canGoForward alimentan el estado de los botones de la barra
// inferior sin repetir las condiciones de paginación en la UI.
func (s *readerState) canGoBack() bool {
	return s.pageIndex > 0 || (s.singlePage && s.spread && s.half == 1)
}

func (s *readerState) canGoForward() bool {
	return s.pageIndex < s.total()-1 || (s.singlePage && s.spread && s.half == 0)
}

// rememberShown guarda la última página realmente dibujada.
func (s *readerState) rememberShown() {
	s.shownComic = s.comic
	s.shownPage = s.pageIndex
	s.shownHalf = s.half
	s.shownSpread = s.spread
}

// restoreShown recupera esa página tras cancelar una carga larga. Si el cómic
// cambió, vuelve al principio.
func (s *readerState) restoreShown() {
	if s.shownComic == s.comic {
		s.pageIndex = s.shownPage
		s.half = s.shownHalf
		s.spread = s.shownSpread
		return
	}
	s.pageIndex = 0
	s.half = 0
	s.spread = false
}

// fitWidth y zoom100 cambian el modo de zoom sin tocar la UI.
func (s *readerState) fitWidth() {
	s.zoomFit = true
}

func (s *readerState) zoom100() {
	s.zoomFit = false
	s.zoomScale = 1
}

// zoomBy aplica un factor sobre el nivel actual y pasa a zoom manual.
func (s *readerState) zoomBy(factor float32) {
	s.zoomFit = false
	s.zoomScale = clampZoom(s.zoomScale * factor)
}

// setZoomScale fija la escala efectiva calculada por applyPageZoom.
func (s *readerState) setZoomScale(scale float32) {
	s.zoomScale = scale
}

// setViewport registra el tamaño del área de lectura. Devuelve true si cambió.
func (s *readerState) setViewport(size fyne.Size) bool {
	if size.Width <= 0 || size.Height <= 0 || size == s.viewport {
		return false
	}
	s.viewport = size
	return true
}

// markShown registra la página que se acaba de dibujar con sus datos reales.
func (s *readerState) markShown(spread bool, half int) {
	s.loading = false
	s.spread = spread
	s.half = half
	s.rememberShown()
}
