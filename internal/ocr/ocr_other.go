//go:build !windows

package ocr

import (
	"context"
	"errors"
)

func platformOCR(context.Context, string, languageOption) ([]ocrLine, error) {
	return nil, errors.New("el OCR integrado solo está disponible en Windows")
}
