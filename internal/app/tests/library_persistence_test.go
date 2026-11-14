package tests

import (
	"fmt"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"gutter/internal/app"
	"gutter/internal/backend"
)

func TestRecentsRoundTripAndLimit(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewBareTestApp(fyneApp.Preferences())
	for i := 0; i < app.MaxRecentComics+5; i++ {
		g.UpsertRecent(app.RecentComic{
			Path:       filepath.Join(t.TempDir(), fmt.Sprintf("%02d.cbz", i)),
			Page:       i,
			Total:      100,
			SinglePage: true,
			ZoomFit:    true,
			ZoomScale:  1,
		})
	}

	recents := g.Recents()
	if len(recents) != app.MaxRecentComics {
		t.Fatalf("la lista en memoria tiene %d entradas, quiero %d", len(recents), app.MaxRecentComics)
	}

	loaded := app.LoadRecents(fyneApp.Preferences())
	if len(loaded) != app.MaxRecentComics {
		t.Fatalf("la lista persistida tiene %d entradas, quiero %d", len(loaded), app.MaxRecentComics)
	}
	if loaded[0].Path != recents[0].Path {
		t.Fatalf("la entrada más reciente no se conservó: %q vs %q", loaded[0].Path, recents[0].Path)
	}
	if loaded[0].Page != recents[0].Page {
		t.Fatalf("la página no se conservó: %d vs %d", loaded[0].Page, recents[0].Page)
	}
}

func TestRestoreProgress(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	comicPath := filepath.Join(t.TempDir(), "Sandman 001.cbz")
	g := app.NewBareTestApp(fyneApp.Preferences())
	g.SetRecents([]app.RecentComic{{
		Path:       comicPath,
		Page:       4,
		Half:       1,
		Total:      10,
		Manga:      true,
		SinglePage: false,
		ZoomFit:    false,
		ZoomScale:  0.8,
	}})
	comic := &backend.Comic{
		Path:  comicPath,
		Pages: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
	}

	g.RestoreProgress(comic)

	if g.PageIndex() != 4 {
		t.Errorf("pageIndex = %d, quiero 4", g.PageIndex())
	}
	if g.Half() != 1 {
		t.Errorf("half = %d, quiero 1", g.Half())
	}
	if !g.Manga() {
		t.Error("se esperaba modo manga")
	}
	if g.SinglePage() {
		t.Error("se esperaba modo de página completa")
	}
	if g.ZoomFit() {
		t.Error("se esperaba zoom manual")
	}
	if g.ZoomScale() != 0.8 {
		t.Errorf("zoomScale = %v, quiero 0.8", g.ZoomScale())
	}
}

func TestSaveCurrentProgressPersistsSession(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	comicPath := filepath.Join(t.TempDir(), "Berserk 001.cbz")
	g := app.NewBareTestApp(fyneApp.Preferences())
	g.SetComic(&backend.Comic{Path: comicPath, Pages: []string{"1", "2", "3", "4", "5"}})
	g.SetPageIndex(3)
	g.SetHalf(1)
	g.SetManga(true)
	g.SetSinglePage(true)
	g.SetZoomFit(false)
	g.SetZoomScale(0.65)

	g.SaveCurrentProgress()

	loaded := app.LoadRecents(fyneApp.Preferences())
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
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	raw := `[{"path":"C:\\Comics\\Uno.cbz","page":99,"half":0,"total":5,"single_page":true,"zoom_fit":true,"zoom_scale":1}]`
	fyneApp.Preferences().SetString(app.PrefRecentComics, raw)

	loaded := app.LoadRecents(fyneApp.Preferences())
	if len(loaded) != 1 {
		t.Fatalf("se esperaba una entrada, hay %d", len(loaded))
	}
	if loaded[0].Page != 4 {
		t.Fatalf("page = %d, quiero 4", loaded[0].Page)
	}
}

func TestRestoreProgressClampsPage(t *testing.T) {
	g := app.NewBareTestApp(nil)
	g.SetRecents([]app.RecentComic{{
		Path:       filepath.Join(t.TempDir(), "corto.cbz"),
		Page:       99,
		Total:      3,
		SinglePage: true,
		ZoomFit:    true,
		ZoomScale:  1,
	}})
	comic := &backend.Comic{Path: g.Recents()[0].Path, Pages: []string{"1", "2", "3"}}

	g.RestoreProgress(comic)
	if g.PageIndex() != 2 {
		t.Fatalf("pageIndex = %d, quiero 2", g.PageIndex())
	}
}
