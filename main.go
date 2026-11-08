// Gutter es un lector de cómics .cbr/.cbz (RAR o ZIP) nativo, rápido y ligero.
//
// main.go es el único punto de entrada de la aplicación. Toda la lógica vive
// en internal/app.
package main

import "gutter/internal/app"

func main() {
	app.Run()
}
