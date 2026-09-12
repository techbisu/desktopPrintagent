//go:build !windows

package printer

import "os/exec"

// setHiddenAttrs is a no-op on non-Windows platforms. It exists only so the
// package compiles for local linting/IDE support; the shipped agent targets
// Windows exclusively, where printer_windows.go's implementation is used.
func setHiddenAttrs(cmd *exec.Cmd) {}
