//go:build !windows

package hardware

import "os/exec"

func setHiddenAttrs(cmd *exec.Cmd) {}
