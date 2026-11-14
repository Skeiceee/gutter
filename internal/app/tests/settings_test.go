package tests

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"gutter/internal/app"
	"gutter/internal/backend"
)

func TestOpenOptionsReflectPreferences(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewBareTestApp(fyneApp.Preferences())
	opts := g.OpenOptions()
	if opts.Sort != backend.SortNatural || opts.IncludeJunk || !opts.IntegrityCheck {
		t.Fatalf("opciones por defecto inesperadas: %+v", opts)
	}

	g.SetPrefString(app.PrefSortMode, string(backend.SortLexicographic))
	g.SetPrefBool(app.PrefFilterJunk, false)
	g.SetPrefBool(app.PrefIntegrityCheck, false)
	opts = g.OpenOptions()
	if opts.Sort != backend.SortLexicographic || !opts.IncludeJunk || opts.IntegrityCheck {
		t.Fatalf("las preferencias no se aplicaron: %+v", opts)
	}
}

func TestDiskLimitLabelsRoundTrip(t *testing.T) {
	for i, label := range app.DiskLimitLabels {
		if got := app.ParseDiskLimit(label); got != app.DiskLimitValues[i] {
			t.Errorf("ParseDiskLimit(%q) = %d, quiero %d", label, got, app.DiskLimitValues[i])
		}
		if got := app.DiskLimitLabel(app.DiskLimitValues[i]); got != label {
			t.Errorf("DiskLimitLabel(%d) = %q, quiero %q", app.DiskLimitValues[i], got, label)
		}
	}
}

func TestCacheSummaryWithoutComic(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	dir := filepath.Join(t.TempDir(), "cache")
	g := app.NewBareTestApp(fyneApp.Preferences())
	g.SetDiskCache(backend.NewDiskCache(dir, 1<<20))
	if got := g.CacheSummary(); got == "" {
		t.Fatal("el resumen de caché no debe estar vacío")
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{2 << 20, "2.0 MB"},
	}
	for _, tc := range cases {
		if got := app.FormatBytes(tc.bytes); got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, quiero %q", tc.bytes, got, tc.want)
		}
	}
}
