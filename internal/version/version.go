// Package version expone la versión de Gutter y utilidades para compararla con
// la última publicación de GitHub.
//
// El valor de Version se inyecta al compilar:
//
//	go build -ldflags "-X gutter/internal/version.Version=1.2.0"
//
// Si no se inyecta (por ejemplo, con `go run .`), String() intenta leer la
// información de compilación de Go y, como último recurso, devuelve "dev".
package version

import (
	"runtime/debug"
	"strconv"
	"strings"
)

// Version es la versión semántica de la compilación actual. Los scripts de
// compilación la sobrescriben con -ldflags.
var Version = "dev"

// String devuelve la versión corta ("1.2.0") o "dev" en compilaciones locales.
func String() string {
	v := strings.TrimSpace(strings.TrimPrefix(Version, "v"))
	if v == "" {
		return "dev"
	}
	return v
}

// Detail añade el commit y la fecha si Go los incrustó en el binario. Se usa en
// «Acerca de» para que un informe de error identifique la compilación exacta.
func Detail() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return String()
	}
	revision, when, modified := "", "", false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			when = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision == "" {
		return String()
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	detail := String() + " (" + revision
	if modified {
		detail += ", con cambios locales"
	}
	if len(when) >= len("2006-01-02") {
		detail += ", " + when[:len("2006-01-02")]
	}
	return detail + ")"
}

// IsDev indica si la compilación no tiene una versión publicada.
func IsDev() bool {
	return String() == "dev"
}

// Compare compara dos versiones semánticas y devuelve -1, 0 o 1. Ignora el
// prefijo "v", los metadatos de compilación (+build) y el sufijo de
// prepublicación (-beta.1), porque Gutter solo publica versiones estables.
func Compare(a, b string) int {
	left, right := parse(a), parse(b)
	size := len(left)
	if len(right) > size {
		size = len(right)
	}
	for i := 0; i < size; i++ {
		var l, r int
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		switch {
		case l < r:
			return -1
		case l > r:
			return 1
		}
	}
	return 0
}

func parse(value string) []int {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "v"))
	if index := strings.IndexAny(value, "-+"); index >= 0 {
		value = value[:index]
	}
	parts := strings.Split(value, ".")
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		digits := part
		if cut := strings.IndexFunc(part, func(r rune) bool {
			return r < '0' || r > '9'
		}); cut >= 0 {
			digits = part[:cut]
		}
		if digits == "" {
			continue
		}
		number, err := strconv.Atoi(digits)
		if err != nil {
			continue
		}
		numbers = append(numbers, number)
	}
	if len(numbers) == 0 {
		return []int{0}
	}
	return numbers
}
