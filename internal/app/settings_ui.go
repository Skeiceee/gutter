package app

// settings_ui.go concentra las preferencias de Gutter en un único diálogo con
// pestañas (Lectura / Caché / Traducción / Interfaz). Antes vivían repartidas
// entre «Opciones de lectura», «Acerca de» y «Configuración de OCR»; tenerlas
// juntas reduce el descubrimiento y evita duplicar validaciones.
//
// El diálogo no aplica nada hasta pulsar «Guardar». Las acciones destructivas
// de la pestaña Caché (vaciar) sí son inmediatas y se confirman con el propio
// estado del diálogo.

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/backend"
	"gutter/internal/ocr"
	"gutter/internal/version"
)

type settingsTab int

const (
	settingsTabReading settingsTab = iota
	settingsTabCache
	settingsTabTranslation
	settingsTabInterface
	settingsTabShortcuts
)

// showSettings abre las preferencias en la pestaña indicada. Un único
// diálogo sustituye a los tres anteriores y mantiene la regla de «solo un
// diálogo a la vez» de showDialog.
func (g *gutterApp) showSettings(tab settingsTab) {
	// --- Lectura ---------------------------------------------------------
	sortSelect := widget.NewSelect(
		[]string{"Natural (página 2 antes que 10)", "Alfabético estricto"},
		nil,
	)
	if g.sortMode() == backend.SortLexicographic {
		sortSelect.SetSelected("Alfabético estricto")
	} else {
		sortSelect.SetSelected("Natural (página 2 antes que 10)")
	}

	junkCheck := widget.NewCheck("Filtrar archivos de sistema (__MACOSX, .DS_Store, ocultos)", nil)
	junkCheck.SetChecked(g.filterJunk())

	integrityCheck := widget.NewCheck("Comprobar integridad al abrir (muestra 3 páginas)", nil)
	integrityCheck.SetChecked(g.integrityCheck())

	thumbSelect := widget.NewSelect(gridThumbLabels, nil)
	thumbSelect.SetSelected(gridThumbLabel(g.gridThumbSizePref()))

	singleCheck := widget.NewCheck("Modo una página (partir las dobles por la mitad)", nil)
	singleCheck.SetChecked(g.reader.singlePage)

	directionSelect := widget.NewSelect([]string{readingWestern, readingManga}, nil)
	if g.reader.manga {
		directionSelect.SetSelected(readingManga)
	} else {
		directionSelect.SetSelected(readingWestern)
	}

	readingTab := container.NewVBox(
		sectionLabel("Paginación"),
		singleCheck,
		widget.NewLabel("Una página: cada doble página se muestra como dos mitades. Desactivado: se ve completa."),
		directionSelect,
		widget.NewLabel("En manga se invierte el sentido de las flechas y de las mitades."),
		widget.NewSeparator(),
		sectionLabel("Orden y limpieza"),
		sortSelect,
		widget.NewLabel("El orden natural interpreta los números: page2 < page10."),
		junkCheck,
		widget.NewLabel("Oculta metadatos y carpetas que macOS/WinRAR dejan dentro del archivo."),
		integrityCheck,
		widget.NewSeparator(),
		sectionLabel("Rejilla de páginas"),
		thumbSelect,
		widget.NewLabel("Se aplica la próxima vez que abras la rejilla con M."),
	)

	// --- Caché -----------------------------------------------------------
	cacheStats := widget.NewLabel(g.cacheSummary())
	cacheStats.Wrapping = fyne.TextWrapWord
	cacheStatus := widget.NewLabel("")
	cacheStatus.Importance = widget.LowImportance

	limitSelect := widget.NewSelect(diskLimitLabels, nil)
	limitSelect.SetSelected(diskLimitLabel(g.diskLimitMB()))

	clearButton := widget.NewButton("Vaciar caché", func() {
		g.clearCaches()
		cacheStats.SetText(g.cacheSummary())
		cacheStatus.SetText("Caché vaciada.")
	})
	clearButton.Importance = widget.LowImportance

	cacheTab := container.NewVBox(
		sectionLabel("Almacenamiento"),
		cacheStats,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Límite de la caché en disco", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		limitSelect,
		widget.NewLabel("Guarda índices y páginas decodificadas para reabrir cómics sin descomprimirlos."),
		container.NewCenter(clearButton),
		cacheStatus,
	)

	// --- Traducción ------------------------------------------------------
	current := ocr.CurrentTranslationConfig(g.prefs)
	sourceSelect := widget.NewSelect(ocr.LanguageLabels(), nil)
	sourceSelect.SetSelected(ocr.LanguageLabel(current.Source))
	targetSelect := widget.NewSelect(ocr.LanguageLabels(), nil)
	targetSelect.SetSelected(ocr.LanguageLabel(current.Target))

	providerSelect := widget.NewSelect(ocr.ProviderLabels(), nil)
	providerSelect.SetSelected(ocr.ProviderByCode(current.Provider).Label)

	endpointEntry := widget.NewEntry()
	endpointEntry.SetText(current.Endpoint)
	endpointEntry.SetPlaceHolder("Vacío = servicio oficial del proveedor")

	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetText(current.APIKey)
	apiKeyEntry.SetPlaceHolder("Solo para LibreTranslate o DeepL")

	ocrOnlyCheck := widget.NewCheck("Solo reconocer el texto (sin traducir)", nil)
	ocrOnlyCheck.SetChecked(current.OCROnly)

	translationHint := widget.NewLabel(
		"El OCR se ejecuta en tu equipo; solo el texto reconocido se envía al " +
			"traductor elegido. MyMemory no necesita clave y tiene un límite " +
			"gratuito diario. Para uso intensivo, configura un servidor " +
			"LibreTranslate o una clave DeepL.\n\n" +
			"En Windows se usa el OCR integrado del sistema; si el idioma no está " +
			"instalado, Gutter intentará Tesseract. En otros sistemas hace falta " +
			"Tesseract en el PATH.",
	)
	translationHint.Wrapping = fyne.TextWrapWord
	translationHint.Importance = widget.LowImportance

	translationForm := widget.NewForm(
		widget.NewFormItem("Idioma del cómic", sourceSelect),
		widget.NewFormItem("Traducir a", targetSelect),
		widget.NewFormItem("Proveedor", providerSelect),
		widget.NewFormItem("Endpoint (LibreTranslate/DeepL)", endpointEntry),
		widget.NewFormItem("Clave API (LibreTranslate/DeepL)", apiKeyEntry),
		widget.NewFormItem("Modo", ocrOnlyCheck),
	)

	translationTab := container.NewVBox(
		sectionLabel("Motor y proveedor"),
		translationForm,
		widget.NewSeparator(),
		translationHint,
	)

	// --- Interfaz --------------------------------------------------------
	rememberWindowCheck := widget.NewCheck("Recordar el tamaño de la ventana entre sesiones", nil)
	rememberWindowCheck.SetChecked(g.rememberWindowSize())

	pageBadgeCheck := widget.NewCheck("Mostrar «Página X / N» en el modo limpio", nil)
	pageBadgeCheck.SetChecked(g.showPageBadgePref())

	checkUpdatesCheck := widget.NewCheck("Buscar actualizaciones al arrancar", nil)
	checkUpdatesCheck.SetChecked(g.checkUpdatesOnStart())

	helpButton := widget.NewButton("Ver atajos de teclado…", g.showHelp)
	helpButton.Importance = widget.LowImportance

	themeSelect := widget.NewSelect([]string{"Oscuro", "Claro"}, nil)
	if g.lightTheme() {
		themeSelect.SetSelected("Claro")
	} else {
		themeSelect.SetSelected("Oscuro")
	}

	interfaceTab := container.NewVBox(
		sectionLabel("Tema"),
		themeSelect,
		widget.NewLabel("Afecta a la interfaz y también al fondo del lienzo de lectura."),
		widget.NewSeparator(),
		sectionLabel("Ventana"),
		rememberWindowCheck,
		pageBadgeCheck,
		container.NewCenter(helpButton),
		widget.NewSeparator(),
		sectionLabel("Actualizaciones"),
		checkUpdatesCheck,
		widget.NewLabel("La búsqueda manual está en Ayuda > Buscar actualizaciones…"),
		widget.NewSeparator(),
		sectionLabel("Acerca de"),
		widget.NewLabelWithStyle("Gutter "+version.Detail(), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Paleta oscura #121212 y clara #DCDCDC, con acento #007ACC."),
	)

	// --- Diálogo ---------------------------------------------------------
	shortcutsTab, keySelects := g.buildShortcutsTab()
	shortcutsScroll := container.NewVScroll(shortcutsTab)
	readingScroll := container.NewVScroll(readingTab)
	cacheScroll := container.NewVScroll(cacheTab)
	translationScroll := container.NewVScroll(translationTab)
	interfaceScroll := container.NewVScroll(interfaceTab)
	for _, scroll := range []*container.Scroll{readingScroll, cacheScroll, translationScroll, interfaceScroll, shortcutsScroll} {
		scroll.SetMinSize(fyne.NewSize(540, 380))
	}

	tabs := container.NewAppTabs(
		container.NewTabItem("Lectura", readingScroll),
		container.NewTabItem("Caché", cacheScroll),
		container.NewTabItem("Traducción", translationScroll),
		container.NewTabItem("Interfaz", interfaceScroll),
		container.NewTabItem("Atajos", shortcutsScroll),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	if tab >= settingsTabReading && tab <= settingsTabShortcuts {
		tabs.SelectTabIndex(int(tab))
	}

	d := dialog.NewCustomConfirm(
		"Preferencias",
		"Guardar",
		"Cancelar",
		tabs,
		func(saved bool) {
			if !saved {
				return
			}
			keys := keySelection(keySelects)
			if err := validateKeymap(keys); err != nil {
				dialog.ShowError(fmt.Errorf("No se guardaron las preferencias. %w", err), g.window)
				return
			}
			g.applySettings(settingsSnapshot{
				sort:           sortSelect.Selected,
				filterJunk:     junkCheck.Checked,
				integrityCheck: integrityCheck.Checked,
				thumbSize:      parseGridThumbSize(thumbSelect.Selected),
				singlePage:     singleCheck.Checked,
				manga:          directionSelect.Selected == readingManga,
				diskLimitMB:    parseDiskLimit(limitSelect.Selected),
				translation:    translationSelection(current, sourceSelect, targetSelect, providerSelect, endpointEntry, apiKeyEntry, ocrOnlyCheck),
				rememberWindow: rememberWindowCheck.Checked,
				pageBadge:      pageBadgeCheck.Checked,
				checkUpdates:   checkUpdatesCheck.Checked,
				lightTheme:     themeSelect.Selected == "Claro",
				keys:           keys,
			})
		},
		g.window,
	)
	d.Resize(fyne.NewSize(640, 600))
	g.showDialog(d)
}

// settingsSnapshot es el valor de todos los controles en el momento de
// guardar. Separarlo del diálogo mantiene la aplicación de cambios legible y
// permite probarla sin widgets.
type settingsSnapshot struct {
	sort           string
	filterJunk     bool
	integrityCheck bool
	thumbSize      int
	singlePage     bool
	manga          bool
	diskLimitMB    int
	translation    ocr.TranslationConfig
	rememberWindow bool
	pageBadge      bool
	checkUpdates   bool
	lightTheme     bool
	keys           map[shortcutAction]fyne.KeyName
}

func translationSelection(current ocr.TranslationConfig, source, target, provider *widget.Select, endpoint, apiKey *widget.Entry, ocrOnly *widget.Check) ocr.TranslationConfig {
	cfg := ocr.TranslationConfig{
		Source:   ocr.LanguageByCode(ocr.LanguageCodeFromLabel(source.Selected, current.Source.Code), current.Source.Code),
		Target:   ocr.LanguageByCode(ocr.LanguageCodeFromLabel(target.Selected, current.Target.Code), current.Target.Code),
		Provider: ocr.ProviderCodeFromLabel(provider.Selected),
		Endpoint: strings.TrimSpace(endpoint.Text),
		APIKey:   strings.TrimSpace(apiKey.Text),
		OCROnly:  ocrOnly.Checked,
	}
	if cfg.OCROnly {
		cfg.Target = cfg.Source
	}
	return cfg
}

// applySettings persiste el snapshot y refresca solo lo que haya cambiado.
func (g *gutterApp) applySettings(next settingsSnapshot) {
	oldLight := g.lightTheme()
	oldSort := g.sortMode()
	oldJunk := g.filterJunk()
	oldIntegrity := g.integrityCheck()
	oldThumbSize := g.gridThumbSizePref()
	oldSingle := g.reader.singlePage
	oldManga := g.reader.manga
	var oldTranslation ocr.TranslationConfig
	if g.prefs != nil {
		oldTranslation = ocr.CurrentTranslationConfig(g.prefs)
	}

	newSort := backend.SortNatural
	if next.sort == "Alfabético estricto" {
		newSort = backend.SortLexicographic
	}

	if g.prefs != nil {
		g.prefs.SetString(prefSortMode, string(newSort))
		g.prefs.SetBool(prefFilterJunk, next.filterJunk)
		g.prefs.SetBool(prefIntegrityCheck, next.integrityCheck)
		g.prefs.SetInt(prefGridThumbSize, next.thumbSize)
		g.prefs.SetInt(prefDiskCacheLimitMB, next.diskLimitMB)
		g.prefs.SetBool(prefRememberWindow, next.rememberWindow)
		g.prefs.SetBool(prefPageBadge, next.pageBadge)
		g.prefs.SetBool(prefCheckUpdates, next.checkUpdates)
		g.prefs.SetString(prefTheme, themeValue(next.lightTheme))
		saveKeymap(g.prefs, next.keys)
		g.prefs.SetString(ocr.PrefOCRSource, next.translation.Source.Code)
		g.prefs.SetString(ocr.PrefTranslateTarget, next.translation.Target.Code)
		g.prefs.SetString(ocr.PrefTranslateProvider, next.translation.Provider)
		g.prefs.SetString(ocr.PrefTranslateEndpoint, next.translation.Endpoint)
		g.prefs.SetString(ocr.PrefTranslateAPIKey, next.translation.APIKey)
		g.prefs.SetBool(ocr.PrefTranslateOCROnly, next.translation.OCROnly)
	}
	if g.diskCache != nil {
		g.diskCache.SetLimit(int64(next.diskLimitMB) << 20)
	}

	g.reader.singlePage = next.singlePage
	g.reader.manga = next.manga
	if oldSingle != next.singlePage || oldManga != next.manga {
		g.reader.half = 0
		g.saveCurrentProgress()
	}

	if g.gridOpen && next.thumbSize != oldThumbSize {
		g.reopenThumbnailGrid()
	}

	if g.reader.comic != nil {
		reindex := newSort != oldSort || next.filterJunk != oldJunk || next.integrityCheck != oldIntegrity
		if reindex {
			g.reloadPreservingPage()
		} else if oldSingle != next.singlePage || oldManga != next.manga {
			g.showPage()
		}
	}

	if next.keys != nil {
		g.keymap = next.keys
	}
	if oldLight != next.lightTheme {
		g.applyTheme(next.lightTheme)
	}
	if oldTranslation != next.translation {
		g.cancelTranslation()
		g.clearTranslation()
	}
	g.updatePageBadge()
	g.refreshMenu()
}

// refreshMenu vuelve a construir el menú para reflejar las marcas de las
// opciones de lectura (una página, dirección).
func (g *gutterApp) refreshMenu() {
	if g.mainMenu == nil {
		return
	}
	g.installMenu()
	// En modo limpio el menú está retirado: installMenu lo vuelve a mostrar,
	// así que se retira de nuevo conservando el objeto para al salir.
	if g.reader.cleanMode {
		g.window.SetMainMenu(nil)
	}
}
