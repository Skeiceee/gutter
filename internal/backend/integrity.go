package backend

// integrity.go implementa la comprobación rápida que se ejecuta al abrir un
// cómic. No recorre todas las páginas: valida la estructura del contenedor y
// muestrea la primera, la central y la última. El presupuesto de tiempo evita
// que un archivo enorme retrase la apertura.
//
// Los problemas encontrados se guardan en la caché de índice para que reabrir
// el cómic no repita el trabajo.

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"path/filepath"
	"time"
)

const integrityBudget = 1200 * time.Millisecond

var errIntegrityBudget = errors.New("presupuesto de integridad agotado")

// quickIntegrityCheck devuelve las entradas que parecen corruptas. La lectura
// nunca se bloquea por estos problemas: solo se avisa al usuario.
func (c *Comic) quickIntegrityCheck() []PageProblem {
	if c == nil || len(c.Pages) == 0 {
		return nil
	}

	deadline := time.Now().Add(integrityBudget)
	problems := make([]PageProblem, 0, 3)
	added := make(map[string]struct{}, 3)
	add := func(name, detail string) {
		if name == "" {
			name = filepath.Base(c.Path)
		}
		if _, duplicate := added[name]; duplicate {
			return
		}
		added[name] = struct{}{}
		problems = append(problems, PageProblem{Name: name, Detail: detail})
	}

	if c.zipFile != nil {
		c.checkZIPIntegrity(deadline, add)
	} else {
		c.checkRARIntegrity(deadline, add)
	}
	c.problems = problems
	return problems
}

func (c *Comic) checkZIPIntegrity(deadline time.Time, add func(name, detail string)) {
	for _, name := range c.Pages {
		f := c.zipFiles[name]
		if f == nil {
			add(name, "la entrada no está en el índice del ZIP")
			continue
		}
		if f.Method != zip.Store && f.Method != zip.Deflate {
			add(name, fmt.Sprintf("método de compresión no soportado (%d)", f.Method))
		}
	}

	for _, name := range samplePageNames(c.Pages) {
		if time.Now().After(deadline) {
			break
		}
		f := c.zipFiles[name]
		if f == nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			add(name, fmt.Sprintf("no se pudo abrir la entrada: %v", err))
			continue
		}
		data, err := readIntegrityEntry(rc, deadline)
		_ = rc.Close()
		if err != nil {
			if errors.Is(err, errIntegrityBudget) {
				break
			}
			add(name, fmt.Sprintf("no se pudo leer la entrada: %v", err))
			continue
		}
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			add(name, fmt.Sprintf("la imagen no se puede decodificar: %v", err))
		}
	}
}

func (c *Comic) checkRARIntegrity(deadline time.Time, add func(name, detail string)) {
	if err := c.ensureRARFiles(); err != nil {
		add(c.Path, fmt.Sprintf("no se pudo releer el índice del RAR: %v", err))
		return
	}

	for _, name := range samplePageNames(c.Pages) {
		if time.Now().After(deadline) {
			break
		}
		f := c.rarFiles[name]
		if f == nil {
			add(name, "la entrada no está en el índice del RAR")
			continue
		}
		// En un RAR sólido abrir una entrada intermedia obliga a descomprimir
		// todas las anteriores. La comprobación se omite para no convertir la
		// apertura en una lectura completa.
		if f.Solid {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			add(name, fmt.Sprintf("no se pudo abrir la entrada: %v", err))
			continue
		}
		data, err := readIntegrityEntry(rc, deadline)
		_ = rc.Close()
		if err != nil {
			if errors.Is(err, errIntegrityBudget) {
				break
			}
			add(name, fmt.Sprintf("no se pudo leer la entrada: %v", err))
			continue
		}
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			add(name, fmt.Sprintf("la imagen no se puede decodificar: %v", err))
		}
	}
}

func samplePageNames(pages []string) []string {
	if len(pages) == 0 {
		return nil
	}
	indices := []int{0, len(pages) / 2, len(pages) - 1}
	out := make([]string, 0, len(indices))
	seen := make(map[int]struct{}, len(indices))
	for _, index := range indices {
		if index < 0 || index >= len(pages) {
			continue
		}
		if _, duplicate := seen[index]; duplicate {
			continue
		}
		seen[index] = struct{}{}
		out = append(out, pages[index])
	}
	return out
}

// readIntegrityEntry lee una entrada completa con límite de tamaño y de tiempo.
func readIntegrityEntry(r io.Reader, deadline time.Time) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(budgetReader{r: r, deadline: deadline}, maxPageBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxPageBytes {
		return nil, fmt.Errorf("la imagen supera el límite de %d MiB", maxPageBytes>>20)
	}
	return data, nil
}

type budgetReader struct {
	r        io.Reader
	deadline time.Time
}

func (r budgetReader) Read(p []byte) (int, error) {
	if !r.deadline.IsZero() && time.Now().After(r.deadline) {
		return 0, errIntegrityBudget
	}
	return r.r.Read(p)
}
