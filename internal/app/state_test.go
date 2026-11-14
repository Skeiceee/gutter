package app

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"gutter/internal/backend"
	"gutter/internal/ocr"
)

func testComic(pages ...string) *backend.Comic {
	return &backend.Comic{Path: "test.cbz", Pages: pages}
}

func TestReaderNextWalksBothHalves(t *testing.T) {
	s := newReaderState()
	s.comic = testComic("1", "2")
	s.spread = true

	if !s.next() || s.pageIndex != 0 || s.half != 1 {
		t.Fatalf("primera llamada: página=%d mitad=%d", s.pageIndex, s.half)
	}
	if !s.next() || s.pageIndex != 1 || s.half != 0 {
		t.Fatalf("segunda llamada: página=%d mitad=%d", s.pageIndex, s.half)
	}
	if s.next() {
		t.Fatal("no debería avanzar más allá de la última página")
	}
}

func TestReaderNextDuringLoadSkipsHalves(t *testing.T) {
	s := newReaderState()
	s.comic = testComic("1", "2", "3")
	s.spread = true
	s.loading = true

	if !s.next() || s.pageIndex != 1 || s.half != 0 {
		t.Fatalf("durante la carga: página=%d mitad=%d", s.pageIndex, s.half)
	}
}

func TestReaderPreviousEntersLastHalf(t *testing.T) {
	s := newReaderState()
	s.comic = testComic("1", "2", "3")
	s.pageIndex = 1

	if !s.previous() || s.pageIndex != 0 || s.half != 1 {
		t.Fatalf("tras retroceder: página=%d mitad=%d", s.pageIndex, s.half)
	}
	if s.previous() {
		t.Fatal("no debería retroceder más allá de la primera página")
	}
}

func TestReaderMangaRightHalf(t *testing.T) {
	s := newReaderState()
	s.manga = false
	s.half = 1
	if !s.isRightHalf() {
		t.Fatal("en occidental la segunda mitad es la derecha")
	}
	s.manga = true
	if s.isRightHalf() {
		t.Fatal("en manga la segunda mitad es la izquierda")
	}
}

func TestReaderGoToClamps(t *testing.T) {
	s := newReaderState()
	s.comic = testComic("1", "2", "3")

	s.goTo(99)
	if s.pageIndex != 2 || s.half != 0 {
		t.Fatalf("goTo(99): página=%d mitad=%d", s.pageIndex, s.half)
	}
	s.goTo(-5)
	if s.pageIndex != 0 {
		t.Fatalf("goTo(-5): página=%d", s.pageIndex)
	}
}

func TestReaderRestoreFromClampsAndKeepsModes(t *testing.T) {
	s := newReaderState()
	entry := recentComic{
		Page:       9,
		Half:       1,
		Manga:      true,
		SinglePage: false,
		ZoomFit:    false,
		ZoomScale:  0.8,
	}

	s.restoreFrom(entry, 3)

	if s.pageIndex != 2 {
		t.Fatalf("página = %d, quiero 2", s.pageIndex)
	}
	if s.half != 1 || !s.manga || s.singlePage {
		t.Fatalf("modos restaurados = %+v", s)
	}
	if s.zoomFit || s.zoomScale != 0.8 {
		t.Fatalf("zoom restaurado = %v/%v", s.zoomFit, s.zoomScale)
	}
}

func TestReaderRestoreShown(t *testing.T) {
	s := newReaderState()
	s.comic = testComic("1", "2", "3")
	s.pageIndex = 1
	s.half = 1
	s.spread = true
	s.rememberShown()

	s.pageIndex = 2
	s.half = 0
	s.spread = false
	s.restoreShown()

	if s.pageIndex != 1 || s.half != 1 || !s.spread {
		t.Fatalf("restoreShown = página=%d mitad=%d spread=%v", s.pageIndex, s.half, s.spread)
	}

	s.comic = testComic("4", "5")
	s.restoreShown()
	if s.pageIndex != 0 || s.spread {
		t.Fatalf("con otro cómic debería volver al principio: %+v", s)
	}
}

func TestReaderViewport(t *testing.T) {
	s := newReaderState()
	if s.setViewport(fyne.NewSize(0, 100)) {
		t.Fatal("un tamaño inválido no debe aceptarse")
	}
	if !s.setViewport(fyne.NewSize(800, 600)) {
		t.Fatal("el primer tamaño válido debe aceptarse")
	}
	if s.setViewport(fyne.NewSize(800, 600)) {
		t.Fatal("repetir el mismo tamaño no debe contar como cambio")
	}
}

func TestApplySettingsPersistsTranslation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	g := &gutterApp{prefs: app.Preferences(), reader: newReaderState()}

	next := settingsSnapshot{
		sort:           "Alfabético estricto",
		filterJunk:     false,
		integrityCheck: false,
		thumbSize:      200,
		singlePage:     false,
		manga:          true,
		diskLimitMB:    256,
		rememberWindow: false,
		pageBadge:      false,
		checkUpdates:   true,
		translation: ocr.TranslationConfig{
			Source:   ocr.LanguageByCode("ja", "en"),
			Target:   ocr.LanguageByCode("es", "es"),
			Provider: "deepl",
			Endpoint: "https://example.test",
			APIKey:   "clave",
		},
	}
	g.applySettings(next)

	if g.sortMode() != backend.SortLexicographic {
		t.Fatalf("sortMode = %v", g.sortMode())
	}
	if g.filterJunk() || g.integrityCheck() {
		t.Fatal("los filtros no se han persistido")
	}
	if g.gridThumbSizePref() != 200 || g.diskLimitMB() != 256 {
		t.Fatal("tamaños no persistidos")
	}
	if g.reader.singlePage || !g.reader.manga {
		t.Fatal("modos de lectura no aplicados")
	}
	if g.rememberWindowSize() || g.showPageBadgePref() || !g.checkUpdatesOnStart() {
		t.Fatal("preferencias de interfaz no aplicadas")
	}

	prefs := app.Preferences()
	if prefs.String(ocr.PrefOCRSource) != "ja" || prefs.String(ocr.PrefTranslateTarget) != "es" {
		t.Fatal("idiomas de traducción no persistidos")
	}
	if prefs.String(ocr.PrefTranslateProvider) != "deepl" || prefs.String(ocr.PrefTranslateAPIKey) != "clave" {
		t.Fatal("proveedor o clave no persistidos")
	}
}
