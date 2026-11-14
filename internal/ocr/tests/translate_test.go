package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gutter/internal/ocr"
)

func TestLanguageMapping(t *testing.T) {
	ja := ocr.LanguageByCode("JA", "en")
	if ja.Code != "ja" || ja.Tesseract != "jpn" || ja.Windows != "ja-JP" {
		t.Fatalf("mapeo de japonés inesperado: %+v", ja)
	}
	if got := ocr.LanguageByCode("no-existe", "es"); got.Code != "es" {
		t.Fatalf("fallback = %q, quiero es", got.Code)
	}
	if got := ocr.LanguageCodeFromLabel(ocr.LanguageLabel(ja), "en"); got != "ja" {
		t.Fatalf("LanguageCodeFromLabel = %q", got)
	}
}

func TestProviderMapping(t *testing.T) {
	if got := ocr.ProviderByCode("DEEPL"); got.Code != "deepl" {
		t.Fatalf("proveedor = %q", got.Code)
	}
	if got := ocr.ProviderByCode("desconocido"); got.Code != "mymemory" {
		t.Fatalf("fallback de proveedor = %q", got.Code)
	}
	if got := ocr.ProviderCodeFromLabel(ocr.ProviderByCode("libretranslate").Label); got != "libretranslate" {
		t.Fatalf("ProviderCodeFromLabel = %q", got)
	}
}

func TestSplitForTranslation(t *testing.T) {
	text := strings.Repeat("palabra ", 100)
	chunks := ocr.SplitForTranslation(text, 100)
	if len(chunks) < 2 {
		t.Fatalf("se esperaban varios fragmentos, hay %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len(chunk) > 100 {
			t.Fatalf("fragmento demasiado largo (%d): %q", len(chunk), chunk)
		}
	}
	if joined := strings.Join(chunks, " "); joined != strings.TrimSpace(text) {
		t.Fatalf("los fragmentos no reconstruyen el texto")
	}
}

func TestLibreTranslateRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("método = %s, quiero POST", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("JSON inválido: %v", err)
		}
		if payload["q"] != "HELLO" || payload["source"] != "en" || payload["target"] != "es" {
			t.Errorf("payload inesperado: %+v", payload)
		}
		_, _ = io.WriteString(w, `{"translatedText":"HOLA"}`)
	}))
	defer server.Close()

	cfg := ocr.TranslationConfig{
		Source:   ocr.LanguageByCode("en", "en"),
		Target:   ocr.LanguageByCode("es", "es"),
		Provider: "libretranslate",
		Endpoint: server.URL,
	}
	got, err := ocr.TranslateWithLibreTranslate(context.Background(), cfg, "HELLO")
	if err != nil {
		t.Fatalf("TranslateWithLibreTranslate: %v", err)
	}
	if got != "HOLA" {
		t.Fatalf("traducción = %q, quiero HOLA", got)
	}
}

func TestDeepLRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("form inválido: %v", err)
		}
		if r.Form.Get("text") != "HELLO" || r.Form.Get("target_lang") != "ES" {
			t.Errorf("form inesperado: %v", r.Form)
		}
		_, _ = io.WriteString(w, `{"translations":[{"text":"HOLA"}]}`)
	}))
	defer server.Close()

	cfg := ocr.TranslationConfig{
		Source:   ocr.LanguageByCode("en", "en"),
		Target:   ocr.LanguageByCode("es", "es"),
		Provider: "deepl",
		Endpoint: server.URL,
		APIKey:   "test-key",
	}
	got, err := ocr.TranslateWithDeepL(context.Background(), cfg, "HELLO")
	if err != nil {
		t.Fatalf("TranslateWithDeepL: %v", err)
	}
	if got != "HOLA" {
		t.Fatalf("traducción = %q, quiero HOLA", got)
	}
}

func TestTranslateBubbleTextOcrOnly(t *testing.T) {
	cfg := ocr.TranslationConfig{
		Source: ocr.LanguageByCode("ja", "en"),
		Target: ocr.LanguageByCode("ja", "en"),
	}
	got, err := ocr.TranslateBubbleText(context.Background(), cfg, "こんにちは")
	if err != nil {
		t.Fatalf("TranslateBubbleText: %v", err)
	}
	if got != "こんにちは" {
		t.Fatalf("texto = %q", got)
	}
}

func TestDeepLCodes(t *testing.T) {
	cases := map[string]string{
		"en": "EN",
		"es": "ES",
		"ja": "JA",
		"pt": "PT-PT",
		"zh": "ZH",
	}
	for code, want := range cases {
		if got := ocr.DeepLCode(code, false); got != want {
			t.Errorf("DeepLCode(%q) = %q, quiero %q", code, got, want)
		}
	}
	if got := ocr.DeepLCode("pt", true); got != "PT" {
		t.Errorf("DeepLCode(pt, source) = %q, quiero PT", got)
	}
}
