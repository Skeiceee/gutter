//go:build !js && !wasm && !android && !ios

package screen

import "github.com/go-gl/glfw/v3.4/glfw"

// primaryWorkArea devuelve el área útil del monitor principal en píxeles de
// pantalla, excluyendo la barra de tareas. Debe llamarse desde el hilo
// principal, después de que Fyne haya inicializado GLFW.
func PrimaryWorkArea() (x, y, width, height int, ok bool) {
	monitor := glfw.GetPrimaryMonitor()
	if monitor == nil {
		return 0, 0, 0, 0, false
	}
	x, y, width, height = monitor.GetWorkarea()
	return x, y, width, height, width > 0 && height > 0
}
