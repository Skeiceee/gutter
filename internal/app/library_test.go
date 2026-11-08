package app

import (
	"path/filepath"
	"testing"
)

func TestRecentPercent(t *testing.T) {
	cases := []struct {
		name  string
		entry recentComic
		want  int
	}{
		{"primera de diez", recentComic{Page: 0, Total: 10}, 10},
		{"mitad", recentComic{Page: 4, Total: 10}, 50},
		{"última", recentComic{Page: 9, Total: 10}, 100},
		{"una sola página", recentComic{Page: 0, Total: 1}, 100},
		{"sin total", recentComic{Page: 3, Total: 0}, 0},
		{"página fuera de rango", recentComic{Page: 99, Total: 10}, 100},
		{"página negativa", recentComic{Page: -5, Total: 10}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := recentPercent(tc.entry); got != tc.want {
				t.Fatalf("recentPercent(%+v) = %d, quiero %d", tc.entry, got, tc.want)
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
		if got := clampPage(tc.page, tc.total); got != tc.want {
			t.Errorf("clampPage(%d, %d) = %d, quiero %d", tc.page, tc.total, got, tc.want)
		}
	}
}

func TestElideLabel(t *testing.T) {
	if got := elideLabel("corto.cbz", 20); got != "corto.cbz" {
		t.Errorf("elideLabel no debe tocar textos cortos: %q", got)
	}
	if got := elideLabel("un-nombre-muy-largo.cbz", 10); got != "un-nombre…" {
		t.Errorf("elideLabel = %q, quiero %q", got, "un-nombre…")
	}
	if got := elideLabel("abc", 1); got != "…" {
		t.Errorf("elideLabel con máximo 1 = %q, quiero …", got)
	}
}

func TestNormalizeComicPathIsAbsoluteAndClean(t *testing.T) {
	got := normalizeComicPath("  comics/../comics/Batman 001.cbz  ")
	if !filepath.IsAbs(got) {
		t.Fatalf("la ruta debe ser absoluta: %q", got)
	}
	if filepath.Clean(got) != got {
		t.Fatalf("la ruta debe estar limpia: %q", got)
	}
	if normalizeComicPath("   ") != "" {
		t.Fatal("una ruta vacía debe quedarse vacía")
	}
}

func TestIsComicPath(t *testing.T) {
	for _, path := range []string{"a.cbr", "B.CBZ", `C:\Comics\Sandman 001.cbr`} {
		if !isComicPath(path) {
			t.Errorf("isComicPath(%q) = false, quiero true", path)
		}
	}
	for _, path := range []string{"a.zip", "a.pdf", "cbr"} {
		if isComicPath(path) {
			t.Errorf("isComicPath(%q) = true, quiero false", path)
		}
	}
}
