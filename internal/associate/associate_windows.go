//go:build windows

package associate

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// canAssociateComicFiles indica si la plataforma permite registrar la
// asociación desde la propia aplicación.
func CanAssociateComicFiles() bool { return true }

// associateComicFiles registra .cbr y .cbz en HKCU\Software\Classes, que no
// necesita privilegios administrativos y solo afecta al usuario actual.
func AssociateComicFiles() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("no se pudo localizar Gutter.exe: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	for _, ext := range []string{".cbr", ".cbz"} {
		if err := setRegistryString(
			`Software\Classes\`+ext,
			"",
			"Gutter.Comic",
		); err != nil {
			return err
		}
	}

	const progID = `Software\Classes\Gutter.Comic`
	if err := setRegistryString(progID, "", "Cómic de Gutter"); err != nil {
		return err
	}
	if err := setRegistryString(progID+`\DefaultIcon`, "", fmt.Sprintf("%q,0", exe)); err != nil {
		return err
	}
	if err := setRegistryString(progID+`\shell\open\command`, "", fmt.Sprintf("%q \"%%1\"", exe)); err != nil {
		return err
	}

	notifyShellAssociationChanged()
	return nil
}

func setRegistryString(path, name, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("no se pudo abrir %s: %w", path, err)
	}
	defer key.Close()
	if err := key.SetStringValue(name, value); err != nil {
		return fmt.Errorf("no se pudo escribir %s: %w", path, err)
	}
	return nil
}

// notifyShellAssociationChanged avisa al Explorador de que las asociaciones
// cambiaron, para que no haga falta reiniciar sesión.
func notifyShellAssociationChanged() {
	const (
		shcneAssocChanged = 0x08000000
		shcnfIDList       = 0x0000
	)
	shell32 := syscall.NewLazyDLL("shell32.dll")
	proc := shell32.NewProc("SHChangeNotify")
	_, _, _ = proc.Call(shcneAssocChanged, shcnfIDList, 0, 0)
}
