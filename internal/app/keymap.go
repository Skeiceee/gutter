package app

// keymap.go define las acciones de lectura que se pueden reasignar y las
// guarda en Preferencias. Esc, F1, Enter y Tab son fijas: cancelan cargas,
// abren la ayuda y navegan por los diálogos, así que no se ofrecen como opción.

import (
	"fmt"

	"fyne.io/fyne/v2"
)

type shortcutAction string

const (
	actNextPage   shortcutAction = "next_page"
	actPrevPage   shortcutAction = "prev_page"
	actFirstPage  shortcutAction = "first_page"
	actLastPage   shortcutAction = "last_page"
	actGotoPage   shortcutAction = "goto_page"
	actThumbGrid  shortcutAction = "thumb_grid"
	actScrollUp   shortcutAction = "scroll_up"
	actScrollDown shortcutAction = "scroll_down"
	actZoomIn     shortcutAction = "zoom_in"
	actZoomOut    shortcutAction = "zoom_out"
	actFitWidth   shortcutAction = "fit_width"
	actZoom100    shortcutAction = "zoom_100"
	actCleanMode  shortcutAction = "clean_mode"
	actWebtoon    shortcutAction = "webtoon"
	actTranslate  shortcutAction = "translate"
	actBookmark   shortcutAction = "bookmark"
)

const prefShortcutPrefix = "keys."

type shortcutSpec struct {
	action     shortcutAction
	label      string
	defaultKey fyne.KeyName
}

// shortcutSpecs fija el orden de las filas en Preferencias y en la ayuda.
var shortcutSpecs = []shortcutSpec{
	{actNextPage, "Página siguiente", fyne.KeyRight},
	{actPrevPage, "Página anterior", fyne.KeyLeft},
	{actFirstPage, "Primera página", fyne.KeyHome},
	{actLastPage, "Última página", fyne.KeyEnd},
	{actGotoPage, "Ir a una página", fyne.KeyG},
	{actThumbGrid, "Rejilla de páginas", fyne.KeyM},
	{actScrollUp, "Desplazar hacia arriba", fyne.KeyUp},
	{actScrollDown, "Desplazar hacia abajo", fyne.KeyDown},
	{actZoomIn, "Aumentar zoom", fyne.KeyPlus},
	{actZoomOut, "Reducir zoom", fyne.KeyMinus},
	{actFitWidth, "Ajustar al ancho", fyne.Key0},
	{actZoom100, "Zoom 100 %", fyne.Key1},
	{actCleanMode, "Modo limpio", fyne.KeyF11},
	{actWebtoon, "Modo tiras (vertical)", fyne.KeyW},
	{actBookmark, "Marcador en esta página", fyne.KeyB},
	{actTranslate, "Traducir página", fyne.KeyT},
}

// keyChoice asocia una tecla reasignable con su nombre visible.
type keyChoice struct {
	name  fyne.KeyName
	label string
}

// keyChoices es la lista cerrada de teclas que se pueden asignar. Se omiten
// Esc, Enter, Tab y F1 porque tienen función fija.
var keyChoices = buildKeyChoices()

func buildKeyChoices() []keyChoice {
	var out []keyChoice
	add := func(name fyne.KeyName, label string) {
		if label == "" {
			label = string(name)
		}
		out = append(out, keyChoice{name: name, label: label})
	}
	for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		add(fyne.KeyName(string(letter)), "")
	}
	for _, digit := range "0123456789" {
		add(fyne.KeyName(string(digit)), "")
	}
	add(fyne.KeyUp, "Flecha arriba")
	add(fyne.KeyDown, "Flecha abajo")
	add(fyne.KeyLeft, "Flecha izquierda")
	add(fyne.KeyRight, "Flecha derecha")
	add(fyne.KeyHome, "Inicio")
	add(fyne.KeyEnd, "Fin")
	add(fyne.KeyPageUp, "Re Pág")
	add(fyne.KeyPageDown, "Av Pág")
	add(fyne.KeySpace, "Espacio")
	add(fyne.KeyPlus, "+")
	add(fyne.KeyMinus, "-")
	add(fyne.KeyEqual, "=")
	add(fyne.KeyComma, ",")
	add(fyne.KeyPeriod, ".")
	add(fyne.KeySlash, "/")
	add(fyne.KeySemicolon, ";")
	add(fyne.KeyF2, "")
	add(fyne.KeyF3, "")
	add(fyne.KeyF4, "")
	add(fyne.KeyF5, "")
	add(fyne.KeyF6, "")
	add(fyne.KeyF7, "")
	add(fyne.KeyF8, "")
	add(fyne.KeyF9, "")
	add(fyne.KeyF10, "")
	add(fyne.KeyF11, "")
	add(fyne.KeyF12, "")
	return out
}

// defaultKeymap devuelve la asignación de fábrica.
func defaultKeymap() map[shortcutAction]fyne.KeyName {
	out := make(map[shortcutAction]fyne.KeyName, len(shortcutSpecs))
	for _, spec := range shortcutSpecs {
		out[spec.action] = spec.defaultKey
	}
	return out
}

// keyLabelFor devuelve el nombre legible de una tecla de la lista.
func keyLabelFor(name fyne.KeyName) string {
	for _, choice := range keyChoices {
		if choice.name == name {
			return choice.label
		}
	}
	return string(name)
}

// keyShortFor devuelve la forma corta de una tecla, pensada para la ayuda y los
// menús. Las flechas se muestran como símbolos.
func keyShortFor(name fyne.KeyName) string {
	switch name {
	case fyne.KeyUp:
		return "↑"
	case fyne.KeyDown:
		return "↓"
	case fyne.KeyLeft:
		return "←"
	case fyne.KeyRight:
		return "→"
	}
	return keyLabelFor(name)
}

// keyNameFromLabel localiza la tecla asociada a un texto de la lista.
func keyNameFromLabel(label string) (fyne.KeyName, bool) {
	for _, choice := range keyChoices {
		if choice.label == label {
			return choice.name, true
		}
	}
	return "", false
}

func isAssignableKey(name fyne.KeyName) bool {
	for _, choice := range keyChoices {
		if choice.name == name {
			return true
		}
	}
	return false
}

// loadKeymap lee las teclas guardadas. Un valor desconocido vuelve al de fábrica.
func loadKeymap(prefs fyne.Preferences) map[shortcutAction]fyne.KeyName {
	out := defaultKeymap()
	if prefs == nil {
		return out
	}
	for _, spec := range shortcutSpecs {
		raw := prefs.String(prefShortcutPrefix + string(spec.action))
		if raw == "" {
			continue
		}
		name := fyne.KeyName(raw)
		if isAssignableKey(name) {
			out[spec.action] = name
		}
	}
	return out
}

// validateKeymap rechaza dos acciones con la misma tecla. Sin esto, una de las
// dos sería inalcanzable.
func validateKeymap(keys map[shortcutAction]fyne.KeyName) error {
	used := make(map[fyne.KeyName]string, len(keys))
	for _, spec := range shortcutSpecs {
		name := keys[spec.action]
		if other, taken := used[name]; taken {
			return fmt.Errorf("«%s» y «%s» usan la misma tecla (%s).", other, spec.label, keyLabelFor(name))
		}
		used[name] = spec.label
	}
	return nil
}

// keyFor devuelve la tecla asignada a una acción.
func (g *gutterApp) keyFor(action shortcutAction) fyne.KeyName {
	if g.keymap != nil {
		if name, ok := g.keymap[action]; ok {
			return name
		}
	}
	return defaultKeymap()[action]
}

// keyText devuelve la tecla de una acción lista para mostrarla en menús y ayuda.
func (g *gutterApp) keyText(action shortcutAction) string {
	return keyShortFor(g.keyFor(action))
}

// actionFor encuentra la acción asociada a una tecla. El signo "=" actúa como
// "+" cuando "+" no tiene otra acción, porque en teclados sin el signo "+"
// directo se pulsa "=" para aumentar el zoom.
func (g *gutterApp) actionFor(name fyne.KeyName) (shortcutAction, bool) {
	for _, spec := range shortcutSpecs {
		if g.keyFor(spec.action) == name {
			return spec.action, true
		}
	}
	if name == fyne.KeyEqual && g.keyFor(actZoomIn) == fyne.KeyPlus {
		return actZoomIn, true
	}
	return "", false
}

// saveKeymap guarda la asignación completa de acciones.
func saveKeymap(prefs fyne.Preferences, keys map[shortcutAction]fyne.KeyName) {
	if prefs == nil {
		return
	}
	for _, spec := range shortcutSpecs {
		prefs.SetString(prefShortcutPrefix+string(spec.action), string(keys[spec.action]))
	}
}
