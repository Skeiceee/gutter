package app

import (
	"fmt"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"gutter/internal/backend"
)

func TestRecentsRoundTripAndLimit(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	g := &gutterApp{prefs: app.Preferences()}
	for i := 0; i < maxRecentComics+5; i++ {
		g.upsertRecent(recentComic{
			Path:       filepath.Join(t.TempDir(), fmt.Sprintf("%02d.cbz", i)),
			Page:       i,
			Total:      100,
			SinglePage: true,
			ZoomFit:    true,
			ZoomScale:  1,
		})
	}

	if len(g.recents) != maxRecentComics {
		t.Fatalf("la lista en memoria tiene %d entradas, quiero %d", len(g.recents), maxRecentComics)
	}

	loaded := loadRecents(app.Preferences())
	if len(loaded) != maxRecentComics {
		t.Fatalf("la lista persistida tiene %d entradas, quiero %d", len(loaded), maxRecentComics)
	}
	if loaded[0].Path != g.recents[0].Path {
		t.Fatalf("la entrada más reciente no se conservó: %q vs %q", loaded[0].Path, g.recents[0].Path)
	}
	if loaded[0].Page != g.recents[0].Page {
		t.Fatalf("la página no se conservó: %d vs %d", loaded[0].Page, g.recents[0].Page)
	}
}

func TestRestoreProgress(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	comicPath := filepath.Join(t.TempDir(), "Sandman 001.cbz")
	g := &gutterApp{
		prefs: app.Preferences(),
		recents: []recentComic{{
			Path:       comicPath,
			Page:       4,
			Half:       1,
			Total:      10,
			Manga:      true,
			SinglePage: false,
			ZoomFit:    false,
			ZoomScale:  0.8,
		}},
	}
	comic := &backend.Comic{
		Path:  comicPath,
		Pages: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
	}

	g.restoreProgress(comic)

	if g.pageIndex != 4 {
		t.Errorf("pageIndex = %d, quiero 4", g.pageIndex)
	}
	if g.half != 1 {
		t.Errorf("half = %d, quiero 1", g.half)
	}
	if !g.manga {
		t.Error("se esperaba modo manga")
	}
	if g.singlePage {
		t.Error("se esperaba modo de página completa")
	}
	if g.zoomFit {
		t.Error("se esperaba zoom manual")
	}
	if g.zoomScale != 0.8 {
		t.Errorf("zoomScale = %v, quiero 0.8", g.zoomScale)
	}
}

func TestSaveCurrentProgressPersistsSession(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	comicPath := filepath.Join(t.TempDir(), "Berserk 001.cbz")
	g := &gutterApp{
		prefs:      app.Preferences(),
		comic:      &backend.Comic{Path: comicPath, Pages: []string{"1", "2", "3", "4", "5"}},
		pageIndex:  3,
		half:       1,
		manga:      true,
		singlePage: true,
		zoomFit:    false,
		zoomScale:  0.65,
	}

	g.saveCurrentProgress()

	loaded := loadRecents(app.Preferences())
	if len(loaded) != 1 {
		t.Fatalf("se esperaba una entrada reciente, hay %d", len(loaded))
	}
	entry := loaded[0]
	if entry.Path != comicPath || entry.Page != 3 || entry.Half != 1 ||
		!entry.Manga || !entry.SinglePage || entry.ZoomFit || entry.ZoomScale != 0.65 {
		t.Fatalf("progreso persistido inesperado: %+v", entry)
	}
}

func TestLoadRecentsClampsStoredPage(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	raw := `[{"path":"C:\\Comics\\Uno.cbz","page":99,"half":0,"total":5,"single_page":true,"zoom_fit":true,"zoom_scale":1}]`
	app.Preferences().SetString(prefRecentComics, raw)

	loaded := loadRecents(app.Preferences())
	if len(loaded) != 1 {
		t.Fatalf("se esperaba una entrada, hay %d", len(loaded))
	}
	if loaded[0].Page != 4 {
		t.Fatalf("page = %d, quiero 4", loaded[0].Page)
	}
}

func TestRestoreProgressClampsPage(t *testing.T) {
	g := &gutterApp{
		recents: []recentComic{{
			Path:       filepath.Join(t.TempDir(), "corto.cbz"),
			Page:       99,
			Total:      3,
			SinglePage: true,
			ZoomFit:    true,
			ZoomScale:  1,
		}},
	}
	comic := &backend.Comic{Path: g.recents[0].Path, Pages: []string{"1", "2", "3"}}

	g.restoreProgress(comic)
	if g.pageIndex != 2 {
		t.Fatalf("pageIndex = %d, quiero 2", g.pageIndex)
	}
}
