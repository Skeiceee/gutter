//go:build windows

package ocr

import (
	"os/exec"
	"syscall"
)

// hideConsole evita que PowerShell o Tesseract abran una consola visible
// cuando Gutter.exe se compila como aplicación GUI.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
