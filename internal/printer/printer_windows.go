//go:build windows

package printer

import (
	"os/exec"
	"syscall"
)

// setHiddenAttrs configures the command to run with no visible window and
// without ever stealing keyboard focus from whatever the shopkeeper is doing.
func setHiddenAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
