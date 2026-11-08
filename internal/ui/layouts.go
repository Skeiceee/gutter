// layouts.go agrupa todos los layouts personalizados de Gutter.
package ui

import (
	"fyne.io/fyne/v2"
)

// pageLayout centra la página dentro del área de lectura.
type PageLayout struct {
	Width, Height float32
}

func (l *PageLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	w, h := l.Width, l.Height
	if w <= 0 || h <= 0 {
		w, h = size.Width, size.Height
	}
	for _, obj := range objects {
		obj.Resize(fyne.NewSize(w, h))
		obj.Move(fyne.NewPos((size.Width-w)/2, (size.Height-h)/2))
	}
}

func (l *PageLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	if l.Width <= 0 || l.Height <= 0 {
		return fyne.NewSize(1, 1)
	}
	return fyne.NewSize(l.Width, l.Height)
}

// rootLayout apila el área de lectura y la barra inferior SIN el padding que
// añade container.NewBorder (theme.Padding, ~10 px lógicos). Ese padding era la
// franja oscura que se veía entre la página y la barra inferior.
type RootLayout struct{}

func (l *RootLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 2 {
		return
	}
	bar := objs[1]
	reader := objs[0]

	barH := float32(0)
	if bar.Visible() {
		barH = bar.MinSize().Height
	}
	if barH < 1 {
		barH = 40
	}
	readerH := size.Height - barH
	if readerH < 1 {
		readerH = 1
	}
	reader.Resize(fyne.NewSize(size.Width, readerH))
	reader.Move(fyne.NewPos(0, 0))
	if bar.Visible() {
		bar.Resize(fyne.NewSize(size.Width, barH))
		bar.Move(fyne.NewPos(0, readerH))
	} else {
		bar.Resize(fyne.NewSize(size.Width, 0))
		bar.Move(fyne.NewPos(0, size.Height))
	}
}

func (l *RootLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) < 2 {
		return fyne.NewSize(1, 1)
	}
	bar := objs[1]
	w := float32(1)
	h := float32(1)
	if bar.Visible() {
		ms := bar.MinSize()
		if ms.Width > w {
			w = ms.Width
		}
		if ms.Height > h {
			h = ms.Height + 1
		}
	}
	if objs[0] != nil {
		ms := objs[0].MinSize()
		if ms.Width > w {
			w = ms.Width
		}
	}
	return fyne.NewSize(w, h)
}

// bottomShadowLayout coloca la sombra del borde inferior pegada a la base del
// área de lectura, con altura fija.
type BottomShadowLayout struct{ Height float32 }

func (l *BottomShadowLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for i, o := range objs {
		if i == len(objs)-1 {
			o.Resize(fyne.NewSize(size.Width, l.Height))
			o.Move(fyne.NewPos(0, size.Height-l.Height))
		} else {
			o.Resize(size)
			o.Move(fyne.NewPos(0, 0))
		}
	}
}

func (l *BottomShadowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(1, 1)
}

// readerLayout es el Stack del área de lectura.
type ReaderLayout struct {
	OnResize func(fyne.Size)
	Last     fyne.Size
}

func (l *ReaderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, obj := range objects {
		obj.Resize(size)
		obj.Move(fyne.NewPos(0, 0))
	}
	if size != l.Last {
		l.Last = size
		if l.OnResize != nil {
			l.OnResize(size)
		}
	}
}

func (l *ReaderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(1, 1)
	for _, obj := range objects {
		if !obj.Visible() {
			continue
		}
		s := obj.MinSize()
		min.Width = fyne.Max(min.Width, s.Width)
		min.Height = fyne.Max(min.Height, s.Height)
	}
	return min
}

// bottomBarLayout coloca la navegación y centra verticalmente cada widget.
type BottomBarLayout struct {
	HPad, VPad float32
	Prev       fyne.CanvasObject
	Center     fyne.CanvasObject
	Next       fyne.CanvasObject
	Zoom       fyne.CanvasObject
	Compact    bool
}

func (l *BottomBarLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	compact := size.Width < 680
	if compact != l.Compact {
		l.Compact = compact
		if l.Zoom != nil {
			if compact {
				l.Zoom.Hide()
			} else {
				l.Zoom.Show()
			}
		}
	}

	left := l.Prev.MinSize()
	right := l.Next.MinSize()
	l.Prev.Resize(left)
	l.Prev.Move(fyne.NewPos(l.HPad, (size.Height-left.Height)/2))
	l.Next.Resize(right)
	l.Next.Move(fyne.NewPos(size.Width-l.HPad-right.Width, (size.Height-right.Height)/2))

	centerWidth := size.Width - 2*l.HPad - left.Width - right.Width
	if centerWidth < 0 {
		centerWidth = 0
	}
	l.Center.Resize(fyne.NewSize(centerWidth, size.Height))
	l.Center.Move(fyne.NewPos(l.HPad+left.Width, 0))
}

func (l *BottomBarLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	left := l.Prev.MinSize()
	right := l.Next.MinSize()
	center := l.Center.MinSize()
	height := fyne.Max(left.Height, fyne.Max(center.Height, right.Height))
	return fyne.NewSize(
		left.Width+center.Width+right.Width+2*l.HPad,
		height+2*l.VPad,
	)
}

// bottomCenterLayout centra el progreso y alinea los controles de zoom a la
// derecha, ambos en el mismo eje vertical.
type BottomCenterLayout struct {
	Progress fyne.CanvasObject
	Zoom     fyne.CanvasObject
}

func (l *BottomCenterLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	gap := float32(0)
	zoomWidth := float32(0)
	if l.Zoom.Visible() {
		gap = 8
		zoomWidth = l.Zoom.MinSize().Width
	}

	progressWidth := size.Width - zoomWidth - gap
	if progressWidth < 0 {
		progressWidth = 0
	}

	p := l.Progress.MinSize()
	l.Progress.Resize(fyne.NewSize(progressWidth, p.Height))
	l.Progress.Move(fyne.NewPos(0, (size.Height-p.Height)/2))

	if zoomWidth > 0 {
		zoom := l.Zoom.MinSize()
		l.Zoom.Resize(zoom)
		l.Zoom.Move(fyne.NewPos(size.Width-zoom.Width, (size.Height-zoom.Height)/2))
	}
}

func (l *BottomCenterLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	p := l.Progress.MinSize()
	return fyne.NewSize(p.Width, fyne.Max(p.Height, l.Zoom.MinSize().Height))
}

// topRightLayout y topCenterLayout viven en hud.go (junto al HUD).
