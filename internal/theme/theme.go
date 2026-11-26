// theme.go define la identidad visual de Gutter.
//
// Hay dos paletas: la oscura (por defecto) y la clara. Ambas son deliberadamente
// de bajo contraste para que la página del cómic sea lo único brillante en
// pantalla. El azul #007ACC se reserva para acciones, foco y selección, igual
// que en un editor de código.
package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// palette agrupa los colores de una variante de Gutter.
type palette struct {
	variant fyne.ThemeVariant

	background     color.NRGBA // lienzo de lectura y fondo general
	panel          color.NRGBA // paneles y barras
	overlay        color.NRGBA // rgb de pastillas flotantes (se combina con alfa)
	button         color.NRGBA // botones
	buttonHover    color.NRGBA // hover
	buttonDisabled color.NRGBA // botón deshabilitado
	accent         color.NRGBA // acción principal, foco y selección de controles
	accentDark     color.NRGBA // pulsado
	text           color.NRGBA // texto principal
	textBright     color.NRGBA // títulos y texto sobre acento
	textMuted      color.NRGBA // placeholders
	border         color.NRGBA // bordes de inputs
	separator      color.NRGBA // separadores
	selection      color.NRGBA // selección de texto
	scrollBar      color.NRGBA // barra de desplazamiento
	scrollBarBack  color.NRGBA // fondo de la barra de desplazamiento
	shadow         color.NRGBA // sombras de elevación
	hyperlink      color.NRGBA
	errorColor     color.NRGBA
	success        color.NRGBA
	warning        color.NRGBA
}

var darkPalette = palette{
	variant:        theme.VariantDark,
	background:     color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xFF}, // negro carbón
	panel:          color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}, // paneles y barras
	overlay:        color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xFF},
	button:         color.NRGBA{R: 0x25, G: 0x25, B: 0x26, A: 0xFF},
	buttonHover:    color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF},
	buttonDisabled: color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF},
	accent:         color.NRGBA{R: 0x00, G: 0x7A, B: 0xCC, A: 0xFF}, // azul VS Code
	accentDark:     color.NRGBA{R: 0x00, G: 0x5A, B: 0x9E, A: 0xFF},
	text:           color.NRGBA{R: 0xE1, G: 0xE1, B: 0xE1, A: 0xFF},
	textBright:     color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	textMuted:      color.NRGBA{R: 0x8A, G: 0x8A, B: 0x8A, A: 0xFF},
	border:         color.NRGBA{R: 0x3C, G: 0x3C, B: 0x3C, A: 0xFF},
	separator:      color.NRGBA{R: 0x2E, G: 0x2E, B: 0x2E, A: 0xFF},
	selection:      color.NRGBA{R: 0x26, G: 0x4F, B: 0x78, A: 0xFF},
	scrollBar:      color.NRGBA{R: 0x4A, G: 0x4A, B: 0x4A, A: 0xFF},
	scrollBarBack:  color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xFF},
	shadow:         color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99},
	hyperlink:      color.NRGBA{R: 0x4D, G: 0xA6, B: 0xFF, A: 0xFF},
	errorColor:     color.NRGBA{R: 0xF1, G: 0x4C, B: 0x4C, A: 0xFF},
	success:        color.NRGBA{R: 0x4E, G: 0xC9, B: 0xB0, A: 0xFF},
	warning:        color.NRGBA{R: 0xCC, G: 0xA7, B: 0x00, A: 0xFF},
}

var lightPalette = palette{
	variant:        theme.VariantLight,
	background:     color.NRGBA{R: 0xDC, G: 0xDC, B: 0xDC, A: 0xFF}, // gris claro del lienzo
	panel:          color.NRGBA{R: 0xF3, G: 0xF3, B: 0xF3, A: 0xFF},
	overlay:        color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	button:         color.NRGBA{R: 0xE6, G: 0xE6, B: 0xE6, A: 0xFF},
	buttonHover:    color.NRGBA{R: 0xD4, G: 0xD4, B: 0xD4, A: 0xFF},
	buttonDisabled: color.NRGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF},
	accent:         color.NRGBA{R: 0x00, G: 0x7A, B: 0xCC, A: 0xFF},
	accentDark:     color.NRGBA{R: 0x00, G: 0x5A, B: 0x9E, A: 0xFF},
	text:           color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF},
	textBright:     color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	textMuted:      color.NRGBA{R: 0x6B, G: 0x6B, B: 0x6B, A: 0xFF},
	border:         color.NRGBA{R: 0xC4, G: 0xC4, B: 0xC4, A: 0xFF},
	separator:      color.NRGBA{R: 0xD0, G: 0xD0, B: 0xD0, A: 0xFF},
	selection:      color.NRGBA{R: 0xAD, G: 0xD6, B: 0xFF, A: 0xFF},
	scrollBar:      color.NRGBA{R: 0xA8, G: 0xA8, B: 0xA8, A: 0xFF},
	scrollBarBack:  color.NRGBA{R: 0xDC, G: 0xDC, B: 0xDC, A: 0xFF},
	shadow:         color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x1F},
	hyperlink:      color.NRGBA{R: 0x00, G: 0x5A, B: 0x9E, A: 0xFF},
	errorColor:     color.NRGBA{R: 0xC5, G: 0x1E, B: 0x1E, A: 0xFF},
	success:        color.NRGBA{R: 0x0E, G: 0x7A, B: 0x5F, A: 0xFF},
	warning:        color.NRGBA{R: 0x8A, G: 0x6D, B: 0x00, A: 0xFF},
}

// active es la paleta usada por los rectángulos dibujados a mano. La fija el
// último tema creado con NewGutterTheme o NewGutterThemeLight.
var active = &darkPalette

// GutterTheme implementa fyne.Theme con una de las dos paletas de Gutter. No se
// consulta la preferencia del sistema: un lector de cómics mantiene su propia
// elección para no alterar la percepción de la página.
type GutterTheme struct {
	p *palette
}

var _ fyne.Theme = (*GutterTheme)(nil)

// NewGutterTheme crea el tema oscuro, el oficial de la aplicación.
func NewGutterTheme() fyne.Theme {
	return newTheme(&darkPalette)
}

// NewGutterThemeLight crea el tema claro.
func NewGutterThemeLight() fyne.Theme {
	return newTheme(&lightPalette)
}

// NewGutterThemeVariant elige la paleta según la preferencia guardada.
func NewGutterThemeVariant(light bool) fyne.Theme {
	if light {
		return NewGutterThemeLight()
	}
	return NewGutterTheme()
}

func newTheme(p *palette) fyne.Theme {
	active = p
	return &GutterTheme{p: p}
}

// Color devuelve los colores de la paleta activa.
func (t *GutterTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	p := t.p
	switch name {
	case theme.ColorNameBackground:
		return p.background

	// Superficies: barras de herramientas, menús, paneles y campos.
	case theme.ColorNameHeaderBackground,
		theme.ColorNameMenuBackground,
		theme.ColorNameOverlayBackground,
		theme.ColorNameInputBackground:
		return p.panel

	case theme.ColorNameButton:
		return p.button
	case theme.ColorNameDisabledButton:
		return p.buttonDisabled
	case theme.ColorNameHover:
		return p.buttonHover
	case theme.ColorNamePressed:
		return p.accentDark
	case theme.ColorNameFocus, theme.ColorNamePrimary:
		return p.accent
	case theme.ColorNameForegroundOnPrimary:
		return p.textBright
	case theme.ColorNameForeground:
		return p.text
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return p.textMuted
	case theme.ColorNameInputBorder, theme.ColorNameInnerWindowBorder:
		return p.border
	case theme.ColorNameInnerWindowBorderInactive, theme.ColorNameSeparator:
		return p.separator
	case theme.ColorNameSelection:
		return p.selection
	case theme.ColorNameScrollBar:
		return p.scrollBar
	case theme.ColorNameScrollBarBackground:
		return p.scrollBarBack
	case theme.ColorNameShadow:
		return p.shadow
	case theme.ColorNameHyperlink:
		return p.hyperlink
	case theme.ColorNameError:
		return p.errorColor
	case theme.ColorNameForegroundOnError:
		return p.textBright
	case theme.ColorNameSuccess:
		return p.success
	case theme.ColorNameForegroundOnSuccess:
		return p.background
	case theme.ColorNameWarning:
		return p.warning
	case theme.ColorNameForegroundOnWarning:
		return p.background
	}

	// Cualquier color nuevo de Fyne cae en la variante base de la paleta.
	return theme.DefaultTheme().Color(name, p.variant)
}

// Font mantiene la tipografía Sans-Serif incluida en Fyne (Noto Sans).
func (t *GutterTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Icon reutiliza el set de iconos de Fyne, que se recolorea con
// ColorNameForeground y funciona en ambas paletas.
func (t *GutterTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size ajusta espaciados y tamaños para una lectura cómoda en escritorio.
func (t *GutterTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 15
	case theme.SizeNameHeadingText:
		return 26
	case theme.SizeNameSubHeadingText:
		return 19
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNamePadding:
		return 10
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameLineSpacing:
		return 6
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInputBorder:
		return 1
	case theme.SizeNameInputRadius,
		theme.SizeNameButtonRadius:
		return 6
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameScrollBarSmall:
		return 8
	case theme.SizeNameSplitThickness:
		return 2
	}

	return theme.DefaultTheme().Size(name)
}

// GutterPanelColor expone el color de paneles de la paleta activa para las
// barras dibujadas a mano.
func GutterPanelColor() color.Color { return active.panel }

// GutterCanvasColor expone el color del lienzo de lectura de la paleta activa.
func GutterCanvasColor() color.Color { return active.background }

// GutterOverlayColor devuelve el color de las pastillas flotantes con la opacidad
// indicada.
func GutterOverlayColor(alpha uint8) color.Color {
	c := active.overlay
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: alpha}
}

// GutterShadowColor devuelve la sombra de la barra inferior de la paleta activa.
func GutterShadowColor() color.Color { return active.shadow }

// IsLight indica si la paleta activa es la clara.
func IsLight() bool { return active.variant == theme.VariantLight }
