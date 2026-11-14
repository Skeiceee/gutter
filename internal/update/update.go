// Package update consulta la última publicación de Gutter en GitHub.
//
// No descarga ni instala nada: solo devuelve los datos del release para que la
// interfaz muestre la versión disponible y abra la página de descarga en el
// navegador del usuario.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultLatestURL es el endpoint público del último release estable.
const DefaultLatestURL = "https://api.github.com/repos/Skeiceee/gutter/releases/latest"

// Release contiene solo los campos que usa la interfaz.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
}

// Version devuelve el tag sin el prefijo "v".
func (r Release) Version() string {
	return strings.TrimSpace(strings.TrimPrefix(r.TagName, "v"))
}

// Client consulta la API de GitHub. HTTP y URL son configurables para poder
// usar un servidor de pruebas.
type Client struct {
	HTTP *http.Client
	URL  string
}

// NewClient crea un cliente con un tiempo límite razonable para una consulta
// que nunca debe bloquear la lectura.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 12 * time.Second},
		URL:  DefaultLatestURL,
	}
}

// Latest obtiene el último release estable publicado.
func (c *Client) Latest(ctx context.Context) (Release, error) {
	var release Release

	url := c.URL
	if url == "" {
		url = DefaultLatestURL
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 12 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return release, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Gutter-Update-Checker")

	resp, err := httpClient.Do(req)
	if err != nil {
		return release, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = resp.Status
		}
		return release, fmt.Errorf("GitHub respondió %s: %s", resp.Status, detail)
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&release); err != nil {
		return release, fmt.Errorf("respuesta de GitHub no válida: %w", err)
	}
	if release.TagName == "" || release.HTMLURL == "" {
		return release, fmt.Errorf("la respuesta de GitHub no incluye versión o enlace de descarga")
	}
	return release, nil
}
