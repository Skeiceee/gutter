//go:build js || wasm || android || ios

package screen

// En plataformas sin GLFW de escritorio el tamaño se deja en manos de Fyne.
func PrimaryWorkArea() (x, y, width, height int, ok bool) {
	return 0, 0, 0, 0, false
}
