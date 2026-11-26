package tests

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"gutter/internal/app"
)

func TestDefaultKeymapHasNoConflicts(t *testing.T) {
	if err := app.ValidateKeymap(app.DefaultKeymap()); err != nil {
		t.Fatalf("la asignación de fábrica no es válida: %v", err)
	}
}

func TestValidateKeymapRejectsDuplicateKeys(t *testing.T) {
	keys := app.DefaultKeymap()
	keys[app.ActBookmark] = fyne.KeyRight

	if err := app.ValidateKeymap(keys); err == nil {
		t.Fatal("se esperaba un error al asignar la misma tecla a dos acciones")
	}
}

func TestLoadKeymapUsesSavedValuesAndIgnoresUnknown(t *testing.T) {
	fyneApp := test.NewApp()
	prefs := fyneApp.Preferences()
	prefs.SetString("keys.bookmark", string(fyne.KeyM))
	prefs.SetString("keys.next_page", "NoExiste")

	keys := app.LoadKeymap(prefs)
	if keys[app.ActBookmark] != fyne.KeyM {
		t.Fatalf("marcador: se esperaba M, se obtuvo %q", keys[app.ActBookmark])
	}
	if keys[app.ActNextPage] != fyne.KeyRight {
		t.Fatalf("tecla desconocida: se esperaba la de fábrica, se obtuvo %q", keys[app.ActNextPage])
	}
}

func TestActionForMapsEqualToZoomInWhenPlusIsBound(t *testing.T) {
	fyneApp := test.NewApp()
	g := app.NewBareTestApp(fyneApp.Preferences())

	action, ok := g.ActionFor(fyne.KeyEqual)
	if !ok || action != app.ActZoomIn {
		t.Fatalf("'=' debería aumentar el zoom, se obtuvo %q (%v)", action, ok)
	}
}

func TestActionForUsesCustomBinding(t *testing.T) {
	fyneApp := test.NewApp()
	g := app.NewBareTestApp(fyneApp.Preferences())
	keys := app.DefaultKeymap()
	keys[app.ActNextPage] = fyne.KeySpace
	g.SetKeymap(keys)

	if action, ok := g.ActionFor(fyne.KeySpace); !ok || action != app.ActNextPage {
		t.Fatalf("espacio debería pasar de página, se obtuvo %q (%v)", action, ok)
	}
	if _, ok := g.ActionFor(fyne.KeyRight); ok {
		t.Fatal("la flecha derecha ya no debería tener acción tras reasignarla")
	}
}

func TestLoadBookmarksDropsInvalidAndDuplicates(t *testing.T) {
	fyneApp := test.NewApp()
	prefs := fyneApp.Preferences()
	prefs.SetString("reader.bookmarks", `[
		{"path":"C:/comics/a.cbz","page":3,"name":"  ","note":""},
		{"path":"C:/comics/a.cbz","page":3,"name":"Duplicado","note":""},
		{"path":"","page":1,"name":"Sin ruta","note":""},
		{"path":"C:/comics/a.cbz","page":-1,"name":"Página negativa","note":""}
	]`)

	entries := app.LoadBookmarks(prefs)
	if len(entries) != 1 {
		t.Fatalf("se esperaba 1 marcador válido, se obtuvieron %d", len(entries))
	}
	if entries[0].Name != "Página 4" {
		t.Fatalf("un nombre vacío debe pasar a 'Página N', se obtuvo %q", entries[0].Name)
	}
}

func TestUpsertBookmarkReplacesSamePage(t *testing.T) {
	fyneApp := test.NewApp()
	g := app.NewBareTestApp(fyneApp.Preferences())

	g.UpsertBookmark(app.Bookmark{Path: "C:/comics/b.cbz", Page: 2, Name: "Primero"})
	g.UpsertBookmark(app.Bookmark{Path: "C:/comics/b.cbz", Page: 2, Name: "Editado", Note: "nota"})

	entries := g.Bookmarks()
	if len(entries) != 1 {
		t.Fatalf("una página debe tener un solo marcador, hay %d", len(entries))
	}
	if entries[0].Name != "Editado" || entries[0].Note != "nota" {
		t.Fatalf("el marcador no se actualizó: %+v", entries[0])
	}
	if reloaded := app.LoadBookmarks(fyneApp.Preferences()); len(reloaded) != 1 {
		t.Fatalf("el marcador debería persistir en preferencias, hay %d", len(reloaded))
	}
}
