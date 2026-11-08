// cursor.go oculta el puntero en el modo limpio. Es un rectángulo transparente
// que implementa desktop.Cursorable: Fyne lo usa para elegir el cursor, pero no
// lo considera Tappable ni Scrollable, así que los clics y la rueda siguen
// llegando a la página.
package cursor

import (
	"image/color"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
)

type CleanCursor struct {
	*canvas.Rectangle
	active func() bool
}

var _ desktop.Cursorable = (*CleanCursor)(nil)

func NewCleanCursor(active func() bool) *CleanCursor {
	return &CleanCursor{
		Rectangle: canvas.NewRectangle(color.Transparent),
		active:    active,
	}
}

func (c *CleanCursor) Cursor() desktop.Cursor {
	if c.active != nil && c.active() {
		return desktop.HiddenCursor
	}
	return desktop.DefaultCursor
}
