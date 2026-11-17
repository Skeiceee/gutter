package backend

import (
	"path"
	"strings"
	"unicode"

	"github.com/nwaples/rardecode/v2"
)

// isComicImage filtra por extensión soportada por los decodificadores.
func isComicImage(name string) bool {
	ext := strings.ToLower(path.Ext(strings.ReplaceAll(name, "\\", "/")))
	_, ok := comicImageExtensions[ext]
	return ok
}

// acceptsEntry aplica el filtro de extensión y, si está activo, el de basura.
func (c *Comic) acceptsEntry(name string) bool {
	if !isComicImage(name) {
		return false
	}
	return c.opts.IncludeJunk || !isJunkComicPath(name)
}

// acceptsRARFile añade a acceptsEntry las reglas propias de RAR: directorios,
// enlaces y el atributo oculto de Windows.
func (c *Comic) acceptsRARFile(f *rardecode.FileHeader) bool {
	if f == nil || f.IsDir || f.LinkType != rardecode.LinkTypeNone {
		return false
	}
	if !c.acceptsEntry(f.Name) {
		return false
	}
	return c.opts.IncludeJunk || !isHiddenComicFile(f)
}

// isJunkComicPath reconoce metadatos y carpetas que no forman parte del cómic.
// Es más amplio que isHiddenComicPath porque incluye archivos de sistema que
// WinRAR y macOS dejan dentro de los contenedores.
func isJunkComicPath(name string) bool {
	name = strings.ReplaceAll(name, "\\", "/")
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			continue
		}
		lower := strings.ToLower(part)
		if strings.HasPrefix(part, ".") || lower == "__macosx" || lower == "@eadir" {
			return true
		}
		switch lower {
		case "thumbs.db", "desktop.ini":
			return true
		}
	}
	return false
}

// isHiddenComicPath ignora archivos ocultos, __MACOSX y metadatos de sistema.
func isHiddenComicPath(name string) bool {
	return isJunkComicPath(name)
}

// isHiddenComicFile combina la convención Unix (.nombre) con el atributo
// FILE_ATTRIBUTE_HIDDEN que WinRAR puede conservar en archivos de Windows.
func isHiddenComicFile(f *rardecode.FileHeader) bool {
	const fileAttributeHidden = 0x2
	if f.HostOS == rardecode.HostOSWindows && f.Attributes&fileAttributeHidden != 0 {
		return true
	}
	return isHiddenComicPath(f.Name)
}

// naturalLess compara nombres de forma alfabética pero numérica en los tramos
// de dígitos: "p2.jpg" < "p10.jpg" y "001.jpg" == "1.jpg" (desempata el nombre).
func naturalLess(a, b string) bool {
	if cmp := naturalCompare(a, b); cmp != 0 {
		return cmp < 0
	}
	return a < b
}

// lexicographicLess compara como cadenas, sin interpretar números. El
// desempate por el nombre original mantiene un orden estable en Windows.
func lexicographicLess(a, b string) bool {
	al, bl := strings.ToLower(a), strings.ToLower(b)
	if al == bl {
		return a < b
	}
	return al < bl
}

func naturalCompare(a, b string) int {
	ar := []rune(strings.ToLower(a))
	br := []rune(strings.ToLower(b))
	i, j := 0, 0

	for i < len(ar) && j < len(br) {
		if unicode.IsDigit(ar[i]) && unicode.IsDigit(br[j]) {
			si, sj := i, j
			for i < len(ar) && unicode.IsDigit(ar[i]) {
				i++
			}
			for j < len(br) && unicode.IsDigit(br[j]) {
				j++
			}

			an := strings.TrimLeft(string(ar[si:i]), "0")
			bn := strings.TrimLeft(string(br[sj:j]), "0")
			if len(an) != len(bn) {
				return sign(len(an) - len(bn))
			}
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
			continue
		}

		if ar[i] != br[j] {
			if ar[i] < br[j] {
				return -1
			}
			return 1
		}
		i++
		j++
	}

	switch {
	case len(ar)-i < len(br)-j:
		return -1
	case len(ar)-i > len(br)-j:
		return 1
	default:
		return 0
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
