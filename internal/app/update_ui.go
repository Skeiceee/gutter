package app

// update_ui.go conecta la comprobación de actualizaciones con la interfaz.
// La consulta se hace en segundo plano y nunca bloquea la lectura; si no hay
// conexión o GitHub falla, la comprobación automática falla en silencio y la
// manual muestra el error.

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/update"
	"gutter/internal/version"
)

const (
	websiteURL    = "https://skeiceee.github.io/gutter/"
	repositoryURL = "https://github.com/Skeiceee/gutter"
)

// maybeCheckUpdates lanza la comprobación de arranque solo si el usuario la ha
// activado en Preferencias > Interfaz.
func (g *gutterApp) maybeCheckUpdates() {
	if g.checkUpdatesOnStart() {
		g.checkForUpdates(false)
	}
}

// checkForUpdates consulta el último release. manual controla si se muestran
// mensajes cuando ya se está actualizado o cuando falla la red.
func (g *gutterApp) checkForUpdates(manual bool) {
	if g.updating {
		return
	}
	g.updating = true

	go func() {
		ctx, cancel := context.WithTimeout(g.baseContext(), 15*time.Second)
		defer cancel()

		release, err := update.NewClient().Latest(ctx)
		fyne.Do(func() {
			g.updating = false
			if err != nil {
				if manual {
					g.hideActiveDialog()
					dialog.ShowError(
						fmt.Errorf("No se pudo comprobar si hay una versión nueva.\n\n%w", err),
						g.window,
					)
				}
				return
			}

			if version.IsDev() {
				if manual {
					g.hideActiveDialog()
					dialog.ShowInformation(
						"Actualizaciones",
						fmt.Sprintf(
							"Estás usando una compilación de desarrollo (%s).\n\n"+
								"La última versión publicada es la %s.",
							version.Detail(), release.Version(),
						),
						g.window,
					)
				}
				return
			}

			if version.Compare(release.Version(), version.String()) <= 0 {
				if manual {
					g.hideActiveDialog()
					dialog.ShowInformation(
						"Actualizaciones",
						fmt.Sprintf("Gutter está actualizado (%s).", version.String()),
						g.window,
					)
				}
				return
			}

			g.showUpdateDialog(release)
		})
	}()
}

// showUpdateDialog presenta la versión disponible con sus notas y un botón que
// abre la página de descarga en el navegador.
func (g *gutterApp) showUpdateDialog(release update.Release) {
	notes := strings.TrimSpace(release.Body)
	if notes == "" {
		notes = "Esta versión no incluye notas."
	}
	runes := []rune(notes)
	if len(runes) > 1500 {
		notes = string(runes[:1500]) + "…"
	}

	title := strings.TrimSpace(release.Name)
	if title == "" {
		title = "Gutter " + release.Version()
	}

	heading := widget.NewLabelWithStyle(
		fmt.Sprintf("Versión %s disponible", release.Version()),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	current := widget.NewLabelWithStyle(
		fmt.Sprintf("Tienes la %s. La página de descarga se abrirá en tu navegador.", version.String()),
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)
	current.Importance = widget.LowImportance

	notesLabel := widget.NewLabel(notes)
	notesLabel.Wrapping = fyne.TextWrapWord
	notesScroll := container.NewVScroll(notesLabel)
	notesScroll.SetMinSize(fyne.NewSize(460, 240))

	downloadButton := widget.NewButtonWithIcon(
		"Descargar "+release.Version(),
		fyneTheme.DownloadIcon(),
		func() { g.openURL(release.HTMLURL) },
	)
	downloadButton.Importance = widget.HighImportance

	content := container.NewVBox(
		heading,
		widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{}),
		current,
		widget.NewSeparator(),
		container.NewCenter(downloadButton),
		notesScroll,
	)

	d := dialog.NewCustom("Actualización disponible", "Cerrar", content, g.window)
	d.Resize(fyne.NewSize(560, 480))
	g.showDialog(d)
}

// openURL abre un enlace en el navegador del sistema.
func (g *gutterApp) openURL(raw string) {
	parsed, err := url.Parse(raw)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Enlace no válido: %w", err), g.window)
		return
	}
	if err := fyne.CurrentApp().OpenURL(parsed); err != nil {
		dialog.ShowError(fmt.Errorf("No se pudo abrir el navegador.\n\n%w", err), g.window)
	}
}
