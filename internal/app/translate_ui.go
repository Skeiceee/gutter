// translate_ui.go conecta el motor de OCR/traducción con la interfaz: activa
// la función bajo demanda, gestiona la caché por página y muestra el diálogo de
// configuración.
package app

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"gutter/internal/ocr"
)

func (g *gutterApp) translationCacheKey(cfg ocr.TranslationConfig) string {
	if g.reader.comic == nil {
		return ""
	}
	return fmt.Sprintf(
		"%s|%d|%d|%t|%t|%s|%s|%s|%s",
		g.reader.comic.Path,
		g.reader.pageIndex,
		g.reader.half,
		g.reader.singlePage,
		g.reader.manga,
		cfg.Source.Code,
		cfg.Target.Code,
		cfg.Provider,
		cfg.Endpoint,
	)
}

// toggleTranslation muestra u oculta la capa traducida. La primera vez
// ejecuta OCR + traducción; después reutiliza la caché de la página.
func (g *gutterApp) toggleTranslation() {
	if g.reader.comic == nil || g.pageImage.Image == nil {
		dialog.ShowInformation("Traducción", "Abre un cómic para traducir una página.", g.window)
		return
	}
	if g.reader.loading || g.translating {
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

// showTranslationSettings abre las preferencias en la pestaña de traducción.
func (g *gutterApp) showTranslationSettings() {
	g.showSettings(settingsTabTranslation)
}
