package app

import (
	"os"
	"strings"

	"fyne.io/fyne/v2/app"
	"gutter/internal/theme"
)

// Run inicia la aplicación Gutter. Punto de entrada llamado desde main.go.
func Run() {
	a := app.NewWithID("io.gutter.reader")

	// Tema fijo: negro carbón + azul eléctrico. Se instala antes de crear la
	// ventana para que todos los widgets se dibujen ya con la paleta Gutter.
	a.Settings().SetTheme(theme.NewGutterTheme())

	ui := NewGutterApp(a)

	// Abrir un cómic desde la terminal o desde una asociación de archivos:
	//   gutter "C:\Comics\Batman 001.cbr"
	initialComic := ""
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		initialComic = os.Args[1]
	}

	// OnStarted se ejecuta cuando Fyne ya conoce el DPI/escala de la ventana.
	// Ajustamos el tamaño al área útil del monitor y, después, abrimos el
	// archivo indicado (si lo hay).
	a.Lifecycle().SetOnStarted(func() {
		ui.FitWindowToPrimaryScreen()
		if initialComic != "" {
			ui.LoadComic(initialComic)
		}
	})

	ui.Run()
}
