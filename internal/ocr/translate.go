// translate.go define los idiomas, la configuración y los proveedores de
// traducción. El OCR es local; solo el texto reconocido sale del equipo cuando
// se usa un proveedor remoto.
package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

const (
	PrefOCRSource         = "ocr.source"
	PrefTranslateTarget   = "translate.target"
	PrefTranslateProvider = "translate.provider"
	PrefTranslateEndpoint = "translate.endpoint"
	PrefTranslateAPIKey   = "translate.apikey"
	PrefTranslateOCROnly  = "translate.ocr_only"
)

// languageOption reúne el código ISO, el nombre visible y los identificadores
// que esperan Tesseract y Windows OCR.
type languageOption struct {
	Code      string
	Label     string
	Tesseract string
	Windows   string
}

var supportedLanguages = []languageOption{
	{Code: "en", Label: "Inglés", Tesseract: "eng", Windows: "en-US"},
	{Code: "es", Label: "Español", Tesseract: "spa", Windows: "es-ES"},
	{Code: "ja", Label: "Japonés", Tesseract: "jpn", Windows: "ja-JP"},
	{Code: "fr", Label: "Francés", Tesseract: "fra", Windows: "fr-FR"},
	{Code: "de", Label: "Alemán", Tesseract: "deu", Windows: "de-DE"},
	{Code: "it", Label: "Italiano", Tesseract: "ita", Windows: "it-IT"},
	{Code: "pt", Label: "Portugués", Tesseract: "por", Windows: "pt-BR"},
	{Code: "ko", Label: "Coreano", Tesseract: "kor", Windows: "ko-KR"},
	{Code: "zh", Label: "Chino simplificado", Tesseract: "chi_sim", Windows: "zh-Hans-CN"},
	{Code: "ru", Label: "Ruso", Tesseract: "rus", Windows: "ru-RU"},
}

type translationProviderOption struct {
	Code  string
	Label string
}

var supportedTranslationProviders = []translationProviderOption{
	{Code: "mymemory", Label: "MyMemory (gratis, sin clave)"},
	{Code: "libretranslate", Label: "LibreTranslate (servidor propio)"},
	{Code: "deepl", Label: "DeepL (requiere clave)"},
}

type TranslationConfig struct {
	Source   languageOption
	Target   languageOption
	Provider string
	Endpoint string
	APIKey   string
	OCROnly  bool
}

type TranslatedBubble struct {
	Box    image.Rectangle
	Source string
	Text   string
}

type PageTranslation struct {
	Bubbles  []TranslatedBubble
	Overlay  *image.RGBA
	Warnings int
}

func LanguageByCode(code, fallback string) languageOption {
	code = strings.ToLower(strings.TrimSpace(code))
	for _, option := range supportedLanguages {
		if option.Code == code {
			return option
		}
	}
	for _, option := range supportedLanguages {
		if option.Code == fallback {
			return option
		}
	}
	return supportedLanguages[0]
}

func LanguageLabel(option languageOption) string {
	return fmt.Sprintf("%s (%s)", option.Label, option.Code)
}

func LanguageLabels() []string {
	labels := make([]string, 0, len(supportedLanguages))
	for _, option := range supportedLanguages {
		labels = append(labels, LanguageLabel(option))
	}
	return labels
}

func LanguageCodeFromLabel(label, fallback string) string {
	for _, option := range supportedLanguages {
		if LanguageLabel(option) == label {
			return option.Code
		}
	}
	return fallback
}

func ProviderByCode(code string) translationProviderOption {
	code = strings.ToLower(strings.TrimSpace(code))
	for _, option := range supportedTranslationProviders {
		if option.Code == code {
			return option
		}
	}
	return supportedTranslationProviders[0]
}

func ProviderLabels() []string {
	labels := make([]string, 0, len(supportedTranslationProviders))
	for _, option := range supportedTranslationProviders {
		labels = append(labels, option.Label)
	}
	return labels
}

func ProviderCodeFromLabel(label string) string {
	for _, option := range supportedTranslationProviders {
		if option.Label == label {
			return option.Code
		}
	}
	return supportedTranslationProviders[0].Code
}

func CurrentTranslationConfig(prefs fyne.Preferences) TranslationConfig {
	source := LanguageByCode(prefs.String(PrefOCRSource), "en")
	target := LanguageByCode(prefs.String(PrefTranslateTarget), "es")
	ocrOnly := prefs.Bool(PrefTranslateOCROnly)
	if ocrOnly {
		target = source
	}
	return TranslationConfig{
		Source:   source,
		Target:   target,
		Provider: ProviderByCode(prefs.String(PrefTranslateProvider)).Code,
		Endpoint: strings.TrimSpace(prefs.String(PrefTranslateEndpoint)),
		APIKey:   strings.TrimSpace(prefs.String(PrefTranslateAPIKey)),
		OCROnly:  ocrOnly,
	}
}

// BuildPageTranslation ejecuta el OCR, agrupa líneas, traduce cada bocadillo y
// compone la capa transparente que se dibuja sobre la página.
func BuildPageTranslation(ctx context.Context, img image.Image, cfg TranslationConfig) (*PageTranslation, error) {
	lines, err := recognizeText(ctx, img, cfg.Source)
	if err != nil {
		return nil, err
	}
	bubbles := groupOCRLines(lines, img.Bounds())
	if len(bubbles) == 0 {
		return nil, errors.New("no se encontró texto en esta página")
	}

	translated, warnings, err := translateBubbles(ctx, cfg, bubbles)
	if err != nil {
		return nil, err
	}
	overlay, err := RenderTranslationOverlay(img.Bounds(), translated)
	if err != nil {
		return nil, err
	}
	return &PageTranslation{
		Bubbles:  translated,
		Overlay:  overlay,
		Warnings: warnings,
	}, nil
}

func translateBubbles(ctx context.Context, cfg TranslationConfig, bubbles []ocrBubble) ([]TranslatedBubble, int, error) {
	results := make([]TranslatedBubble, len(bubbles))
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	var mu sync.Mutex
	failures := 0
	var firstErr error

	for i, bubble := range bubbles {
		wg.Add(1)
		go func(index int, b ocrBubble) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				failures++
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				mu.Unlock()
				results[index] = TranslatedBubble{Box: b.Rect, Source: b.Text, Text: b.Text}
				return
			}

			translated, err := translateBubbleText(ctx, cfg, b.Text)
			if err != nil {
				mu.Lock()
				failures++
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				translated = b.Text
				if containsCJK(b.Text) {
					// Mejor dejar el bocadillo original que dibujar tofu.
					translated = ""
				}
			}
			results[index] = TranslatedBubble{Box: b.Rect, Source: b.Text, Text: translated}
		}(i, bubble)
	}
	wg.Wait()

	if failures == len(bubbles) && firstErr != nil {
		return nil, failures, firstErr
	}
	return results, failures, nil
}

func translateBubbleText(ctx context.Context, cfg TranslationConfig, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if cfg.Source.Code == cfg.Target.Code {
		return text, nil
	}

	switch cfg.Provider {
	case "libretranslate":
		return translateWithLibreTranslate(ctx, cfg, text)
	case "deepl":
		return translateWithDeepL(ctx, cfg, text)
	default:
		return translateWithMyMemory(ctx, cfg, text)
	}
}

var translationHTTPClient = &http.Client{Timeout: 25 * time.Second}

func translateWithMyMemory(ctx context.Context, cfg TranslationConfig, text string) (string, error) {
	chunks := splitForTranslation(text, 450)
	translated := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		values := url.Values{}
		values.Set("q", chunk)
		values.Set("langpair", cfg.Source.Code+"|"+cfg.Target.Code)

		request, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			"https://api.mymemory.translated.net/get?"+values.Encode(),
			nil,
		)
		if err != nil {
			return "", err
		}
		request.Header.Set("User-Agent", "Gutter/0.2 (comic reader)")
		response, err := translationHTTPClient.Do(request)
		if err != nil {
			return "", fmt.Errorf("MyMemory no respondió: %w", err)
		}
		body, readErr := readLimitedBody(response.Body)
		_ = response.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		if response.StatusCode != http.StatusOK {
			return "", fmt.Errorf("MyMemory devolvió HTTP %d", response.StatusCode)
		}

		var payload struct {
			ResponseData struct {
				TranslatedText string `json:"translatedText"`
			} `json:"responseData"`
			ResponseStatus  int    `json:"responseStatus"`
			ResponseDetails string `json:"responseDetails"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", fmt.Errorf("respuesta inválida de MyMemory: %w", err)
		}
		result := strings.TrimSpace(html.UnescapeString(payload.ResponseData.TranslatedText))
		if payload.ResponseStatus != http.StatusOK || result == "" {
			detail := strings.TrimSpace(payload.ResponseDetails)
			if detail == "" {
				detail = "sin detalle"
			}
			return "", fmt.Errorf("MyMemory rechazó la traducción: %s", detail)
		}
		upper := strings.ToUpper(result)
		if strings.Contains(upper, "MYMEMORY WARNING") || strings.Contains(upper, "QUERY LENGTH LIMIT") {
			return "", fmt.Errorf("MyMemory alcanzó su límite gratuito: %s", result)
		}
		translated = append(translated, result)
	}
	return strings.Join(translated, " "), nil
}

func translateWithLibreTranslate(ctx context.Context, cfg TranslationConfig, text string) (string, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "https://libretranslate.com/translate"
	}

	payload := map[string]any{
		"q":      text,
		"source": cfg.Source.Code,
		"target": cfg.Target.Code,
		"format": "text",
	}
	if cfg.APIKey != "" {
		payload["api_key"] = cfg.APIKey
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Gutter/0.2 (comic reader)")
	response, err := translationHTTPClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("LibreTranslate no respondió: %w", err)
	}
	responseBody, readErr := readLimitedBody(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return "", readErr
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LibreTranslate devolvió HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var result struct {
		TranslatedText string `json:"translatedText"`
		Error          string `json:"error"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", fmt.Errorf("respuesta inválida de LibreTranslate: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("LibreTranslate: %s", result.Error)
	}
	if strings.TrimSpace(result.TranslatedText) == "" {
		return "", errors.New("LibreTranslate devolvió una traducción vacía")
	}
	return strings.TrimSpace(result.TranslatedText), nil
}

func translateWithDeepL(ctx context.Context, cfg TranslationConfig, text string) (string, error) {
	if cfg.APIKey == "" {
		return "", errors.New("DeepL necesita una clave API; configúrala en Traducción > Configuración")
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "https://api-free.deepl.com/v2/translate"
	}

	form := url.Values{}
	form.Set("auth_key", cfg.APIKey)
	form.Set("text", text)
	form.Set("source_lang", deepLCode(cfg.Source.Code, true))
	form.Set("target_lang", deepLCode(cfg.Target.Code, false))

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "Gutter/0.2 (comic reader)")
	response, err := translationHTTPClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("DeepL no respondió: %w", err)
	}
	body, readErr := readLimitedBody(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return "", readErr
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepL devolvió HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("respuesta inválida de DeepL: %w", err)
	}
	if len(payload.Translations) == 0 || strings.TrimSpace(payload.Translations[0].Text) == "" {
		return "", errors.New("DeepL devolvió una traducción vacía")
	}
	return strings.TrimSpace(payload.Translations[0].Text), nil
}

func deepLCode(code string, source bool) string {
	switch strings.ToLower(code) {
	case "pt":
		if source {
			return "PT"
		}
		return "PT-PT"
	case "zh":
		return "ZH"
	default:
		return strings.ToUpper(code)
	}
}

func splitForTranslation(text string, maxBytes int) []string {
	if len(text) <= maxBytes {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []string
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if len(candidate) <= maxBytes {
			current = candidate
			continue
		}
		if current != "" {
			chunks = append(chunks, current)
		}
		current = word
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

func readLimitedBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la respuesta del traductor: %w", err)
	}
	return data, nil
}
