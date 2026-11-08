// translate_ui.go conecta el motor de OCR/traducción con la interfaz: activa
// la función bajo demanda, gestiona la caché por página y muestra el diálogo de
// configuración.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"gutter/internal/ocr"
)

func (g *gutterApp) translationCacheKey(cfg ocr.TranslationConfig) string {
	if g.comic == nil {
		return ""
	}
	return fmt.Sprintf(
		"%s|%d|%d|%t|%t|%s|%s|%s|%s",
		g.comic.Path,
		g.pageIndex,
		g.half,
		g.singlePage,
		g.manga,
		cfg.Source.Code,
		cfg.Target.Code,
		cfg.Provider,
		cfg.Endpoint,
	)
}

// toggleTranslation muestra u oculta la capa traducida. La primera vez
// ejecuta OCR + traducción; después reutiliza la caché de la página.
func (g *gutterApp) toggleTranslation() {
	if g.comic == nil || g.pageImage.Image == nil {
		dialog.ShowInformation("Traducción", "Abre un cómic para traducir una página.", g.window)
		return
	}
	if g.loading || g.translating {
		return
	}
	if g.translationVisible {
		g.clearTranslation()
		g.unfocus()
		return
	}
	g.startTranslation()
	g.unfocus()
}

func (g *gutterApp) startTranslation() {
	cfg := ocr.CurrentTranslationConfig(g.prefs)
	key := g.translationCacheKey(cfg)
	if cached, ok := g.translationCache[key]; ok {
		g.showTranslation(cached)
		return
	}

	img := g.pageImage.Image
	seq := g.translateSeq.Add(1)
	g.translating = true
	g.progress.SetText("OCR + traducción…")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		result, err := ocr.BuildPageTranslation(ctx, img, cfg)
		fyne.Do(func() {
			if seq != g.translateSeq.Load() {
				return // la página cambió mientras se procesaba
			}
			g.translating = false
			g.updateControls()
			if err != nil {
				dialog.ShowError(fmt.Errorf("No se pudo traducir la página.\n\n%w", err), g.window)
				return
			}
			if g.translationCache == nil {
				g.translationCache = make(map[string]*ocr.PageTranslation)
			}
			if len(g.translationCache) >= 12 {
				g.translationCache = make(map[string]*ocr.PageTranslation)
			}
			// Una traducción parcial no se cachea: al volver a pulsar T se
			// reintentan los bocadillos que fallaron. La caché guarda solo el
			// texto y las cajas; la capa RGBA se regenera al mostrarla para no
			// acumular decenas de MB por página.
			if result.Warnings == 0 {
				cached := *result
				cached.Overlay = nil
				g.translationCache[key] = &cached
			}
			g.showTranslation(result)
			if result.Warnings > 0 {
				dialog.ShowInformation(
					"Traducción parcial",
					fmt.Sprintf(
						"%d bocadillo(s) no se pudieron traducir y se han dejado como estaban. "+
							"Puedes revisar la conexión o el proveedor en Traducción > Configuración.",
						result.Warnings,
					),
					g.window,
				)
			}
		})
	}()
}

func (g *gutterApp) showTranslation(result *ocr.PageTranslation) {
	if result == nil || g.translationImage == nil || g.pageImage.Image == nil {
		return
	}
	overlay := result.Overlay
	if overlay == nil {
		var err error
		overlay, err = ocr.RenderTranslationOverlay(g.pageImage.Image.Bounds(), result.Bubbles)
		if err != nil {
			dialog.ShowError(fmt.Errorf("No se pudo dibujar la traducción.\n\n%w", err), g.window)
			return
		}
	}
	g.translationImage.Image = overlay
	g.translationImage.Show()
	g.translationVisible = true
	g.pageCanvas.Refresh()
}

func (g *gutterApp) clearTranslation() {
	if g.translationImage == nil {
		return
	}
	g.translationImage.Image = nil
	g.translationImage.Hide()
	g.translationVisible = false
	g.pageCanvas.Refresh()
}

func (g *gutterApp) showTranslationSettings() {
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

	hint := widget.NewLabel(
		"El OCR se ejecuta en tu equipo; solo el texto reconocido se envía al " +
			"traductor elegido. MyMemory no necesita clave y tiene un límite " +
			"gratuito diario. Para uso intensivo, configura un servidor " +
			"LibreTranslate o una clave DeepL.\n\n" +
			"En Windows se usa el OCR integrado del sistema; si el idioma no está " +
			"instalado, Gutter intentará Tesseract. En otros sistemas hace falta " +
			"Tesseract en el PATH.",
	)
	hint.Wrapping = fyne.TextWrapWord
	hint.Importance = widget.LowImportance

	form := widget.NewForm(
		widget.NewFormItem("Idioma del cómic", sourceSelect),
		widget.NewFormItem("Traducir a", targetSelect),
		widget.NewFormItem("Proveedor", providerSelect),
		widget.NewFormItem("Endpoint (LibreTranslate/DeepL)", endpointEntry),
		widget.NewFormItem("Clave API (LibreTranslate/DeepL)", apiKeyEntry),
		widget.NewFormItem("Modo", ocrOnlyCheck),
	)
	content := container.NewVBox(form, widget.NewSeparator(), hint)
	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(520, 360))

	d := dialog.NewCustomConfirm(
		"OCR y traducción",
		"Guardar",
		"Cancelar",
		scroll,
		func(saved bool) {
			if !saved {
				return
			}
			sourceCode := ocr.LanguageCodeFromLabel(sourceSelect.Selected, current.Source.Code)
			targetCode := ocr.LanguageCodeFromLabel(targetSelect.Selected, current.Target.Code)
			g.prefs.SetString(ocr.PrefOCRSource, sourceCode)
			g.prefs.SetString(ocr.PrefTranslateTarget, targetCode)
			g.prefs.SetString(ocr.PrefTranslateProvider, ocr.ProviderCodeFromLabel(providerSelect.Selected))
			g.prefs.SetString(ocr.PrefTranslateEndpoint, strings.TrimSpace(endpointEntry.Text))
			g.prefs.SetString(ocr.PrefTranslateAPIKey, strings.TrimSpace(apiKeyEntry.Text))
			g.prefs.SetBool(ocr.PrefTranslateOCROnly, ocrOnlyCheck.Checked)

			// Si cambian idioma o proveedor, la capa anterior ya no es válida.
			g.translateSeq.Add(1)
			g.translating = false
			g.clearTranslation()
		},
		g.window,
	)
	d.Resize(fyne.NewSize(600, 560))
	g.showDialog(d)
}
