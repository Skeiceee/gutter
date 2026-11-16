//go:build !race

package tests

// open_async_test.go comprueba de extremo a extremo que la apertura de un cómic
// no bloquea y que solo se instala el resultado vigente.
//
// Se excluye en modo -race: el driver de pruebas de Fyne ejecuta fyne.Do en la
// misma goroutine que lo llama (el resto de drivers lo encola en el hilo de
// UI), así que este flujo tocaría widgets desde dos goroutines y el detector de
// carreras reportaría falsos positivos del propio driver.

import (
	"archive/zip"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"gutter/internal/app"
)

func TestLoadComicOpensInBackground(t *testing.T) {
	fyneApp := test.NewApp()
	defer fyneApp.Quit()

	g := app.NewTestApp(fyneApp)
	defer g.CloseComic()

	first := writeTestComic(t, "comic.cbz", 2)

	g.LoadComic(first)
	if !g.Opening() {
		t.Fatal("loadComic debe marcar la apertura en curso antes de volver")
	}

	waitForProgress(t, g, "Página 1 de 2")
	if comic := g.Comic(); comic == nil || len(comic.Pages) != 2 {
		t.Fatalf("cómic instalado = %+v", comic)
	}
	if g.Opening() {
		t.Fatal("la apertura debe haber terminado")
	}

	// Abrir otro cómic sustituye al anterior y libera el archivo viejo.
	second := writeTestComic(t, "second.cbz", 3)
	g.LoadComic(second)
	waitForProgress(t, g, "Página 1 de 3")
	if comic := g.Comic(); comic == nil || comic.Path != second || len(comic.Pages) != 3 {
		t.Fatalf("el segundo cómic no quedó instalado: %+v", comic)
	}
}

// waitForProgress espera a que la barra inferior muestre el texto esperado.
func waitForProgress(t *testing.T, g *app.TestApp, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(g.ProgressText(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("la barra no llegó a mostrar %q: %q", want, g.ProgressText())
}

// writeTestComic crea un .cbz con páginas PNG diminutas.
func writeTestComic(t *testing.T, name string, pages int) string {
	t.Helper()

	comicPath := filepath.Join(t.TempDir(), name)
	f, err := os.Create(comicPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	img := image.NewRGBA(image.Rect(0, 0, 20, 30))
	for i := 0; i < pages; i++ {
		w, err := zw.Create(fmt.Sprintf("page%02d.png", i))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(w, img); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return comicPath
}
