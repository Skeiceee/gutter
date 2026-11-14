package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatestParsesRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Error("falta User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.3.0",
			"name": "Gutter 1.3.0",
			"html_url": "https://github.com/Skeiceee/gutter/releases/tag/v1.3.0",
			"body": "Novedades",
			"published_at": "2026-11-14T10:00:00Z"
		}`))
	}))
	defer server.Close()

	client := &Client{HTTP: server.Client(), URL: server.URL}
	release, err := client.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if release.Version() != "1.3.0" {
		t.Fatalf("Version() = %q, quiero 1.3.0", release.Version())
	}
	if release.HTMLURL == "" || release.Name != "Gutter 1.3.0" {
		t.Fatalf("release incompleto: %+v", release)
	}
	if release.PublishedAt.IsZero() {
		t.Fatal("published_at no se ha interpretado")
	}
}

func TestLatestReportsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sin permisos", http.StatusForbidden)
	}))
	defer server.Close()

	client := &Client{HTTP: server.Client(), URL: server.URL}
	if _, err := client.Latest(context.Background()); err == nil {
		t.Fatal("esperaba error con HTTP 403")
	}
}

func TestLatestRejectsIncompleteRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name": "v1.3.0"}`))
	}))
	defer server.Close()

	client := &Client{HTTP: server.Client(), URL: server.URL}
	if _, err := client.Latest(context.Background()); err == nil {
		t.Fatal("esperaba error si falta html_url")
	}
}
