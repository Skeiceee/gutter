package tests

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"gutter/internal/app"
	"gutter/internal/backend"
	"gutter/internal/ocr"
)

func testComic(pages ...string) *backend.Comic {
	return &backend.Comic{Path: "test.cbz", Pages: pages}
}

func TestReaderNextWalksBothHalves(t *testing.T) {
	s := app.NewReader()
	s.SetComic(testComic("1", "2"))
	s.SetSpread(true)

	if !s.Next() || s.Page() != 0 || s.Half() != 1 {
		t.Fatalf("primera llamada: página=%d mitad=%d", s.Page(), s.Half())
	}
	if !s.Next() || s.Page() != 1 || s.Half() != 0 {
		t.Fatalf("segunda llamada: página=%d mitad=%d", s.Page(), s.Half())
	}
	if s.Next() {
		t.Fatal("no debería avanzar más allá de la última página")
	}
}

func TestReaderNextDuringLoadSkipsHalves(t *testing.T) {
	s := app.NewReader()
	s.SetComic(testComic("1", "2", "3"))
	s.SetSpread(true)
	s.SetLoading(true)

	if !s.Next() || s.Page() != 1 || s.Half() != 0 {
		t.Fatalf("durante la carga: página=%d mitad=%d", s.Page(), s.Half())
	}
}

func TestReaderPreviousEntersLastHalf(t *testing.T) {
	s := app.NewReader()
	s.SetComic(testComic("1", "2", "3"))
	s.SetPage(1)

	if !s.Previous() || s.Page() != 0 || s.Half() != 1 {
		t.Fatalf("tras retroceder: página=%d mitad=%d", s.Page(), s.Half())
	}
	if s.Previous() {
		t.Fatal("no debería retroceder más allá de la primera página")
	}
}

func TestReaderMangaRightHalf(t *testing.T) {
	s := app.NewReader()
	s.SetHalf(1)
	if !s.IsRightHalf() {
		t.Fatal("en occidental la segunda mitad es la derecha")
	}
	s.SetManga(true)
	if s.IsRightHalf() {
		t.Fatal("en manga la segunda mitad es la izquierda")
	}
}

func TestReaderGoToClamps(t *testing.T) {
	s := app.NewReader()
	s.SetComic(testComic("1", "2", "3"))

	s.GoTo(99)
	if s.Page() != 2 || s.Half() != 0 {
		t.Fatalf("GoTo(99): página=%d mitad=%d", s.Page(), s.Half())
	}
	s.GoTo(-5)
	if s.Page() != 0 {
		t.Fatalf("GoTo(-5): página=%d", s.Page())
	}
}

func TestReaderRestoreFromClampsAndKeepsModes(t *testing.T) {
	s := app.NewReader()
	entry := app.RecentComic{
		Page:       9,
		Half:       1,
		Manga:      true,
		SinglePage: false,
		ZoomFit:    false,
		ZoomScale:  0.8,
	}

	s.RestoreFrom(entry, 3)

	if s.Page() != 2 {
		t.Fatalf("página = %d, quiero 2", s.Page())
	}
	if s.Half() != 1 || !s.Manga() || s.SinglePage() {
		t.Fatalf("modos restaurados = página=%d mitad=%d manga=%v unaPágina=%v",
			s.Page(), s.Half(), s.Manga(), s.SinglePage())
	}
	if s.ZoomFit() || s.ZoomScale() != 0.8 {
		t.Fatalf("zoom restaurado = %v/%v", s.ZoomFit(), s.ZoomScale())
	}
}

func TestReaderRestoreShown(t *testing.T) {
	s := app.NewReader()
	s.SetComic(testComic("1", "2", "3"))
	s.SetPage(1)
	s.SetHalf(1)
	s.SetSpread(true)
	s.RememberShown()

	s.SetPage(2)
	s.SetHalf(0)
	s.SetSpread(false)
	s.RestoreShown()

	if s.Page() != 1 || s.Half() != 1 || !s.Spread() {
		t.Fatalf("RestoreShown = página=%d mitad=%d spread=%v", s.Page(), s.Half(), s.Spread())
	}

	s.SetComic(testComic("4", "5"))
	s.RestoreShown()
	if s.Page() != 0 || s.Spread() {
		t.Fatalf("con otro cómic debería volver al principio: página=%d spread=%v", s.Page(), s.Spread())
	}
}

func TestReaderViewport(t *testing.T) {
	s := app.NewReader()
	if s.SetViewport(fyne.NewSize(0, 100)) {
		t.Fatal("un tamaño inválido no debe aceptarse")
	}
	if !s.SetViewport(fyne.NewSize(800, 600)) {
		t.Fatal("el primer tamaño válido debe aceptarse")
	}
	if s.SetViewport(fyne.NewSize(800, 600)) {
		t.Fatal("repetir el mismo tamaño no debe contar como cambio")
	}
}

func TestApplySettingsPersistsTranslation(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewBareTestApp(fyneApp.Preferences())

	g.ApplySettings(app.SettingsSnapshot{
		Sort:           "Alfabético estricto",
		FilterJunk:     false,
		IntegrityCheck: false,
		ThumbSize:      200,
		SinglePage:     false,
		Manga:          true,
		DiskLimitMB:    256,
		RememberWindow: false,
		PageBadge:      false,
		CheckUpdates:   true,
		Translation: ocr.TranslationConfig{
			Source:   ocr.LanguageByCode("ja", "en"),
			Target:   ocr.LanguageByCode("es", "es"),
			Provider: "deepl",
			Endpoint: "https://example.test",
			APIKey:   "clave",
		},
	})

	if g.SortMode() != backend.SortLexicographic {
		t.Fatalf("SortMode = %v", g.SortMode())
	}
	if g.FilterJunk() || g.IntegrityCheck() {
		t.Fatal("los filtros no se han persistido")
	}
	if g.GridThumbSizePref() != 200 || g.DiskLimitMB() != 256 {
		t.Fatal("tamaños no persistidos")
	}
	if g.Reader().SinglePage() || !g.Reader().Manga() {
		t.Fatal("modos de lectura no aplicados")
	}
	if g.RememberWindowSize() || g.ShowPageBadgePref() || !g.CheckUpdatesOnStart() {
		t.Fatal("preferencias de interfaz no aplicadas")
	}

	prefs := g.Prefs()
	if prefs.String(ocr.PrefOCRSource) != "ja" || prefs.String(ocr.PrefTranslateTarget) != "es" {
		t.Fatal("idiomas de traducción no persistidos")
	}
	if prefs.String(ocr.PrefTranslateProvider) != "deepl" || prefs.String(ocr.PrefTranslateAPIKey) != "clave" {
		t.Fatal("proveedor o clave no persistidos")
	}
}
