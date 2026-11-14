// help.go agrupa el menú de la ventana, la hoja de atajos y los diálogos
// informativos. Mantener estos textos en un solo archivo facilita traducirlos o
// ampliarlos sin tocar la lógica de lectura.
package app

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/associate"
	"gutter/internal/version"
)

// installMenu crea el menú nativo de la ventana. Los atajos con Ctrl se
// registran aquí para que funcionen también cuando el foco está en un control.
func (g *gutterApp) installMenu() {
	openItem := fyne.NewMenuItem("Abrir cómic…", g.openFile)
	openItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl}

	recentItem := fyne.NewMenuItem("Recientes…", g.showRecents)
	recentItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl}

	fileItems := []*fyne.MenuItem{openItem, recentItem}
	if associate.CanAssociateComicFiles() {
		fileItems = append(fileItems,
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Asociar .cbr/.cbz con Gutter…", g.confirmAssociateFiles),
		)
	}
	salirItem := fyne.NewMenuItem("Salir", func() { g.window.Close() })
	salirItem.IsQuit = true
	fileItems = append(fileItems,
		fyne.NewMenuItemSeparator(),
		salirItem,
	)

	preferencesItem := fyne.NewMenuItem("Preferencias…", func() { g.showSettings(settingsTabReading) })
	preferencesItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyComma, Modifier: fyne.KeyModifierControl}

	singlePageItem := fyne.NewMenuItem("Modo una página", func() {
		g.setSinglePage(!g.reader.singlePage)
	})
	singlePageItem.Checked = g.reader.singlePage

	westernItem := fyne.NewMenuItem("Dirección: Occidental (I→D)", func() {
		g.setReadingDirection(false)
	})
	westernItem.Checked = !g.reader.manga

	mangaItem := fyne.NewMenuItem("Dirección: Manga (D→I)", func() {
		g.setReadingDirection(true)
	})
	mangaItem.Checked = g.reader.manga

	viewItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Modo limpio (F11)", g.toggleCleanMode),
		fyne.NewMenuItem("Rejilla de páginas (M)", g.toggleThumbnailGrid),
		fyne.NewMenuItem("Ir a página… (G)", g.openGotoPage),
		fyne.NewMenuItemSeparator(),
		singlePageItem,
		westernItem,
		mangaItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Ajustar al ancho (0)", g.fitWidth),
		fyne.NewMenuItem("Zoom 100 % (1)", g.zoom100),
		fyne.NewMenuItemSeparator(),
		preferencesItem,
	}

	translationItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Traducir / ocultar página (T)", g.toggleTranslation),
		fyne.NewMenuItem("Preferencias de OCR y traducción…", func() { g.showSettings(settingsTabTranslation) }),
	}

	helpItems := []*fyne.MenuItem{
		fyne.NewMenuItem("Atajos de teclado…", g.showHelp),
		fyne.NewMenuItem("Buscar actualizaciones…", func() { g.checkForUpdates(false) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Acerca de Gutter", g.showAbout),
	}

	g.mainMenu = fyne.NewMainMenu(
		fyne.NewMenu("Archivo", fileItems...),
		fyne.NewMenu("Ver", viewItems...),
		fyne.NewMenu("Traducción", translationItems...),
		fyne.NewMenu("Ayuda", helpItems...),
	)
	g.window.SetMainMenu(g.mainMenu)
}

func (g *gutterApp) showAbout() {
	logo := canvas.NewImageFromResource(g.logo)
	logo.FillMode = canvas.ImageFillContain
	logo.ScaleMode = canvas.ImageScaleSmooth
	logo.SetMinSize(fyne.NewSize(260, 100))

	title := widget.NewLabelWithStyle("Gutter", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	versionLabel := widget.NewLabelWithStyle(
		"Versión "+version.Detail(),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	description := widget.NewLabelWithStyle(
		"Lector de cómics .cbr/.cbz de código abierto.\n"+
			"RAR v4/v5 y ZIP con descompresión selectiva, caché LRU y progreso persistente.",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)
	description.Wrapping = fyne.TextWrapWord

	websiteButton := widget.NewButton("Sitio web", func() { g.openURL(websiteURL) })
	repositoryButton := widget.NewButton("Repositorio", func() { g.openURL(repositoryURL) })
	updateButton := widget.NewButtonWithIcon("Buscar actualizaciones", fyneTheme.ViewRefreshIcon(), func() {
		g.checkForUpdates(false)
	})
	updateButton.Importance = widget.HighImportance

	links := container.NewGridWithColumns(
		2,
		websiteButton,
		repositoryButton,
	)

	content := container.NewVBox(
		container.NewCenter(logo),
		container.NewCenter(title),
		versionLabel,
		description,
		widget.NewSeparator(),
		links,
		container.NewCenter(updateButton),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(
			"Caché, lectura, traducción e interfaz se ajustan en Preferencias (Ctrl+,).",
			fyne.TextAlignCenter,
			fyne.TextStyle{Italic: true},
		),
	)

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(460, 340))
	d := dialog.NewCustom("Acerca de Gutter", "Cerrar", scroll, g.window)
	d.Resize(fyne.NewSize(540, 500))
	g.showDialog(d)
}

// showHelp presenta la hoja de referencia. Se llega aquí con F1, con "?" o
// desde el menú Ayuda.
func (g *gutterApp) showHelp() {
	body := container.NewVBox(
		sectionLabel("Lectura"),
		shortcutRow("→ / ←", "Página siguiente / anterior (se invierten en manga)"),
		shortcutRow("↑ / ↓", "Desplazar la página verticalmente"),
		shortcutRow("Inicio / Fin", "Primera / última página"),
		shortcutRow("G + número + Enter", "Ir a una página concreta"),
		shortcutRow("M", "Abrir o cerrar la rejilla de páginas"),
		shortcutRow("↑↓←→ + Enter", "Elegir una miniatura y abrirla"),
		sectionLabel("Zoom"),
		shortcutRow("+ / −", "Aumentar o reducir el zoom"),
		shortcutRow("0", "Ajustar la página al ancho"),
		shortcutRow("1", "Zoom 100 % (1 píxel = 1 unidad Fyne)"),
		sectionLabel("Ventana"),
		shortcutRow("F11", "Modo limpio a pantalla completa (indicador de página arriba a la derecha)"),
		shortcutRow("Esc", "Cancelar una carga larga / salir del modo limpio"),
		shortcutRow("F1 o ?", "Mostrar esta ayuda"),
		sectionLabel("Traducción"),
		shortcutRow("T", "Reconocer y traducir la página actual"),
		shortcutRow("T (otra vez)", "Ocultar la capa de traducción"),
		shortcutRow("Traducción > Configuración", "Idioma, proveedor y clave API"),
		sectionLabel("Archivo"),
		shortcutRow("Ctrl+O", "Abrir un cómic"),
		shortcutRow("Ctrl+R", "Cambiar a un cómic reciente"),
		shortcutRow("Arrastrar y soltar", "Abrir un .cbr/.cbz sobre la ventana"),
		shortcutRow("Rueda", "Desplazar la página en ambos ejes"),
	)

	scroll := container.NewVScroll(body)
	scroll.SetMinSize(fyne.NewSize(460, 360))
	d := dialog.NewCustom("Atajos de Gutter", "Cerrar", scroll, g.window)
	d.Resize(fyne.NewSize(540, 500))
	g.showDialog(d)
}

func sectionLabel(text string) fyne.CanvasObject {
	label := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.New(layout.NewCustomPaddedLayout(12, 2, 0, 0), label)
}

func shortcutRow(key, description string) fyne.CanvasObject {
	keyLabel := widget.NewLabelWithStyle(key, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	keyBox := container.NewGridWrap(fyne.NewSize(155, 28), keyLabel)
	desc := widget.NewLabel(description)
	desc.Wrapping = fyne.TextWrapWord
	return container.NewBorder(nil, nil, keyBox, nil, desc)
}

// confirmAssociateFiles pide permiso antes de tocar el registro de Windows.
func (g *gutterApp) confirmAssociateFiles() {
	d := dialog.NewConfirm(
		"Asociar archivos",
		"Gutter se registrará como aplicación para abrir archivos .cbr y .cbz "+
			"en tu usuario de Windows. No requiere permisos de administrador.\n\n"+
			"¿Quieres continuar?",
		func(confirmed bool) {
			if !confirmed {
				return
			}
			if err := associate.AssociateComicFiles(); err != nil {
				g.hideActiveDialog()
				dialog.ShowError(fmt.Errorf("no se pudo crear la asociación: %w", err), g.window)
				return
			}
			g.hideActiveDialog()
			dialog.ShowInformation(
				"Asociación creada",
				"Windows ya ofrece Gutter para abrir .cbr y .cbz. Si el tipo de archivo "+
					"ya tenía otra aplicación predeterminada, elige Gutter desde "+
					"«Abrir con» una sola vez.",
				g.window,
			)
		},
		g.window,
	)
	g.showDialog(d)
}
