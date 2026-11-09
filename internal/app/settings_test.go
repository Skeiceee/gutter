package app

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"gutter/internal/backend"
)

func TestOpenOptionsReflectPreferences(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	g := &gutterApp{prefs: app.Preferences()}
	opts := g.openOptions()
	if opts.Sort != backend.SortNatural || opts.IncludeJunk || !opts.IntegrityCheck {
		t.Fatalf("opciones por defecto inesperadas: %+v", opts)
	}

	g.prefs.SetString(prefSortMode, string(backend.SortLexicographic))
	g.prefs.SetBool(prefFilterJunk, false)
	g.prefs.SetBool(prefIntegrityCheck, false)
	opts = g.openOptions()
	if opts.Sort != backend.SortLexicographic || !opts.IncludeJunk || opts.IntegrityCheck {
		t.Fatalf("las preferencias no se aplicaron: %+v", opts)
	}
}

func TestDiskLimitLabelsRoundTrip(t *testing.T) {
	for i, label := range diskLimitLabels {
		if got := parseDiskLimit(label); got != diskLimitValues[i] {
			t.Errorf("parseDiskLimit(%q) = %d, quiero %d", label, got, diskLimitValues[i])
		}
		if got := diskLimitLabel(diskLimitValues[i]); got != label {
			t.Errorf("diskLimitLabel(%d) = %q, quiero %q", diskLimitValues[i], got, label)
		}
	}
}

func TestCacheSummaryWithoutComic(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	dir := filepath.Join(t.TempDir(), "cache")
	g := &gutterApp{
		prefs:     app.Preferences(),
		diskCache: backend.NewDiskCache(dir, 1<<20),
	}
	if got := g.cacheSummary(); got == "" {
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
		if got := formatBytes(tc.bytes); got != tc.want {
			t.Errorf("formatBytes(%d) = %q, quiero %q", tc.bytes, got, tc.want)
		}
	}
}
