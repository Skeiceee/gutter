//go:build !windows

package associate

import "errors"

func CanAssociateComicFiles() bool { return false }

func AssociateComicFiles() error {
	return errors.New("la asociación automática solo está disponible en Windows")
}
