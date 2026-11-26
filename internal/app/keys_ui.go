package app

// keys_ui.go construye la pestaña "Atajos" de Preferencias. Cada acción tiene un
// desplegable con las teclas disponibles. Se eligen de una lista en lugar de
// capturar la pulsación, porque capturar teclas dentro de un diálogo modal es
// frágil y no todas las teclas llegan al canvas.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func keyChoiceLabels() []string {
	labels := make([]string, 0, len(keyChoices))
	for _, choice := range keyChoices {
		labels = append(labels, choice.label)
	}
	return labels
}

// buildShortcutsTab devuelve la pestaña y los desplegables por acción, para
// leer sus valores al guardar.
func (g *gutterApp) buildShortcutsTab() (fyne.CanvasObject, map[shortcutAction]*widget.Select) {
	labels := keyChoiceLabels()
	selects := make(map[shortcutAction]*widget.Select, len(shortcutSpecs))
	items := make([]*widget.FormItem, 0, len(shortcutSpecs))
	for _, spec := range shortcutSpecs {
		sel := widget.NewSelect(labels, nil)
		sel.SetSelected(keyLabelFor(g.keyFor(spec.action)))
		selects[spec.action] = sel
		items = append(items, widget.NewFormItem(spec.label, sel))
	}

	reset := widget.NewButton("Restablecer valores predeterminados", func() {
		for _, spec := range shortcutSpecs {
			selects[spec.action].SetSelected(keyLabelFor(spec.defaultKey))
		}
	})
	reset.Importance = widget.LowImportance

	hint := widget.NewLabel(
		"Esc, Enter, Tab y F1 tienen función fija y no se pueden asignar. " +
			"Si dos acciones usan la misma tecla, los cambios no se guardan.",
	)
	hint.Wrapping = fyne.TextWrapWord
	hint.Importance = widget.LowImportance

	body := container.NewVBox(
		sectionLabel("Teclas de lectura"),
		widget.NewForm(items...),
		hint,
		container.NewCenter(reset),
	)
	return body, selects
}

// keySelection lee las teclas elegidas. Un valor no reconocido vuelve al de
// fábrica para no guardar algo inválido.
func keySelection(selects map[shortcutAction]*widget.Select) map[shortcutAction]fyne.KeyName {
	out := defaultKeymap()
	for _, spec := range shortcutSpecs {
		sel, ok := selects[spec.action]
		if !ok {
			continue
		}
		if name, found := keyNameFromLabel(sel.Selected); found {
			out[spec.action] = name
		}
	}
	return out
}
