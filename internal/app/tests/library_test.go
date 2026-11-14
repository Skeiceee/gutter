package tests

import (
	"path/filepath"
	"testing"

	"gutter/internal/app"
)

func TestRecentPercent(t *testing.T) {
	cases := []struct {
		name  string
		entry app.RecentComic
		want  int
	}{
		{"primera de diez", app.RecentComic{Page: 0, Total: 10}, 10},
		{"mitad", app.RecentComic{Page: 4, Total: 10}, 50},
		{"última", app.RecentComic{Page: 9, Total: 10}, 100},
		{"una sola página", app.RecentComic{Page: 0, Total: 1}, 100},
		{"sin total", app.RecentComic{Page: 3, Total: 0}, 0},
		{"página fuera de rango", app.RecentComic{Page: 99, Total: 10}, 100},
		{"página negativa", app.RecentComic{Page: -5, Total: 10}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := app.RecentPercent(tc.entry); got != tc.want {
				t.Fatalf("RecentPercent(%+v) = %d, quiero %d", tc.entry, got, tc.want)
			}
		})
	}
}

func TestClampPage(t *testing.T) {
	cases := []struct {
		page, total, want int
	}{
		{0, 10, 0},
		{9, 10, 9},
		{10, 10, 9},
		{-1, 10, 0},
		{3, 1, 0},
	}
	for _, tc := range cases {
		if got := app.ClampPage(tc.page, tc.total); got != tc.want {
			t.Errorf("ClampPage(%d, %d) = %d, quiero %d", tc.page, tc.total, got, tc.want)
		}
	}
}

func TestElideLabel(t *testing.T) {
	if got := app.ElideLabel("corto.cbz", 20); got != "corto.cbz" {
		t.Errorf("ElideLabel no debe tocar textos cortos: %q", got)
	}
	if got := app.ElideLabel("un-nombre-muy-largo.cbz", 10); got != "un-nombre…" {
		t.Errorf("ElideLabel = %q, quiero %q", got, "un-nombre…")
	}
	if got := app.ElideLabel("abc", 1); got != "…" {
		t.Errorf("ElideLabel con máximo 1 = %q, quiero …", got)
	}
}

func TestNormalizeComicPathIsAbsoluteAndClean(t *testing.T) {
	got := app.NormalizeComicPath("  comics/../comics/Batman 001.cbz  ")
	if !filepath.IsAbs(got) {
		t.Fatalf("la ruta debe ser absoluta: %q", got)
	}
	if filepath.Clean(got) != got {
		t.Fatalf("la ruta debe estar limpia: %q", got)
	}
	if app.NormalizeComicPath("   ") != "" {
		t.Fatal("una ruta vacía debe quedarse vacía")
	}
}

func TestIsComicPath(t *testing.T) {
	for _, path := range []string{"a.cbr", "B.CBZ", `C:\Comics\Sandman 001.cbr`} {
		if !app.IsComicPath(path) {
			t.Errorf("IsComicPath(%q) = false, quiero true", path)
		}
	}
	for _, path := range []string{"a.zip", "a.pdf", "cbr"} {
		if app.IsComicPath(path) {
			t.Errorf("IsComicPath(%q) = true, quiero false", path)
		}
	}
}
