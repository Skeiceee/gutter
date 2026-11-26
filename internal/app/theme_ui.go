package app

// theme_ui.go aplica el tema elegido en Preferencias. Fyne repinta sus widgets
// solo, pero los rectángulos dibujados a mano (lienzo, barras, pastillas) fijan
// su color al crearse. Por eso se registran aquí y se repintan al cambiar de tema.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	gtheme "gutter/internal/theme"
)

const prefTheme = "ui.theme"

// themedSurface relaciona un rectángulo con la función que da su color actual.
type themedSurface struct {
	rect *canvas.Rectangle
	tint func() color.Color
}

func canvasTint() color.Color { return gtheme.GutterCanvasColor() }
func panelTint() color.Color  { return gtheme.GutterPanelColor() }
func pillTint() color.Color   { return gtheme.GutterOverlayColor(0xE8) }
func gridTint() color.Color   { return gtheme.GutterOverlayColor(0xF4) }

// themedRect crea un rectángulo que sigue al tema activo.
func (g *gutterApp) themedRect(tint func() color.Color) *canvas.Rectangle {
	rect := canvas.NewRectangle(tint())
	g.surfaces = append(g.surfaces, themedSurface{rect: rect, tint: tint})
	return rect
}

// lightTheme indica si la preferencia guardada es el tema claro.
func (g *gutterApp) lightTheme() bool {
	return g.prefs != nil && g.prefs.String(prefTheme) == "light"
}

func themeValue(light bool) string {
	if light {
		return "light"
	}
	return "dark"
}

// applyTheme cambia la paleta de Fyne y repinta las superficies dibujadas a mano.
func (g *gutterApp) applyTheme(light bool) {
	fyne.CurrentApp().Settings().SetTheme(gtheme.NewGutterThemeVariant(light))
	for _, s := range g.surfaces {
		s.rect.FillColor = s.tint()
		s.rect.Refresh()
	}
	g.refreshStripColors()
	if g.root != nil {
		g.root.Refresh()
	}
}
