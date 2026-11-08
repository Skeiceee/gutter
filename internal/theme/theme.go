// theme.go define la identidad visual de Gutter.
//
// La paleta es deliberadamente oscura y de bajo contraste para que la página
// del cómic sea lo único brillante en pantalla. El azul #007ACC se reserva para
// acciones, foco y selección, igual que en un editor de código.
package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Colores corporativos de Gutter.
var (
	colorBackground  = color.NRGBA{R: 0x12, G: 0x12, B: 0x12, A: 0xFF} // negro carbón
	colorPanel       = color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF} // paneles y barras
	colorButton      = color.NRGBA{R: 0x25, G: 0x25, B: 0x26, A: 0xFF} // botones
	colorButtonHover = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF} // hover
	colorAccent      = color.NRGBA{R: 0x00, G: 0x7A, B: 0xCC, A: 0xFF} // azul VS Code
	colorAccentDark  = color.NRGBA{R: 0x00, G: 0x5A, B: 0x9E, A: 0xFF} // pulsado
	colorText        = color.NRGBA{R: 0xE1, G: 0xE1, B: 0xE1, A: 0xFF} // texto principal
	colorTextBright  = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // títulos
	colorTextMuted   = color.NRGBA{R: 0x8A, G: 0x8A, B: 0x8A, A: 0xFF} // placeholders
	colorBorder      = color.NRGBA{R: 0x3C, G: 0x3C, B: 0x3C, A: 0xFF} // bordes de inputs
	colorSeparator   = color.NRGBA{R: 0x2E, G: 0x2E, B: 0x2E, A: 0xFF} // separadores
	colorSelection   = color.NRGBA{R: 0x26, G: 0x4F, B: 0x78, A: 0xFF} // selección de texto
)

// GutterTheme implementa fyne.Theme con variante oscura fija.
// No se consulta la preferencia del sistema: un lector de cómics siempre debe
// mantener el fondo negro para no alterar la percepción de la página.
type GutterTheme struct{}

var _ fyne.Theme = (*GutterTheme)(nil)

// NewGutterTheme crea el tema oficial de la aplicación.
func NewGutterTheme() fyne.Theme {
	return &GutterTheme{}
}

// Color devuelve los colores de la paleta Gutter.
func (t *GutterTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colorBackground

	// Superficies: barras de herramientas, menús, paneles y campos.
	case theme.ColorNameHeaderBackground,
		theme.ColorNameMenuBackground,
		theme.ColorNameOverlayBackground,
		theme.ColorNameInputBackground:
		return colorPanel

	case theme.ColorNameButton:
		return colorButton
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0x1E, G: 0x1E, B: 0x1E, A: 0xFF}
	case theme.ColorNameHover:
		return colorButtonHover
	case theme.ColorNamePressed:
		return colorAccentDark
	case theme.ColorNameFocus:
		return colorAccent
	case theme.ColorNamePrimary:
		return colorAccent
	case theme.ColorNameForegroundOnPrimary:
		return colorTextBright
	case theme.ColorNameForeground:
		return colorText
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return colorTextMuted
	case theme.ColorNameInputBorder, theme.ColorNameInnerWindowBorder:
		return colorBorder
	case theme.ColorNameInnerWindowBorderInactive:
		return colorSeparator
	case theme.ColorNameSeparator:
		return colorSeparator
	case theme.ColorNameSelection:
		return colorSelection
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0x4A, G: 0x4A, B: 0x4A, A: 0xFF}
	case theme.ColorNameScrollBarBackground:
		return colorBackground
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99}
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 0x4D, G: 0xA6, B: 0xFF, A: 0xFF}
	case theme.ColorNameError:
		return color.NRGBA{R: 0xF1, G: 0x4C, B: 0x4C, A: 0xFF}
	case theme.ColorNameForegroundOnError:
		return colorTextBright
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0x4E, G: 0xC9, B: 0xB0, A: 0xFF}
	case theme.ColorNameForegroundOnSuccess:
		return colorBackground
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xCC, G: 0xA7, B: 0x00, A: 0xFF}
	case theme.ColorNameForegroundOnWarning:
		return colorBackground
	}

	// Cualquier color nuevo de Fyne cae en la paleta oscura oficial.
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

// Font mantiene la tipografía Sans-Serif incluida en Fyne (Noto Sans).
func (t *GutterTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Icon reutiliza el set de iconos de Fyne, que ya está diseñado para fondos
// oscuros y se recolorea con ColorNameForeground.
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

// gutterPanelColor expone el gris de paneles para las barras dibujadas a mano.
func GutterPanelColor() color.Color { return colorPanel }

// gutterCanvasColor expone el negro carbón del lienzo de lectura.
func GutterCanvasColor() color.Color { return colorBackground }
