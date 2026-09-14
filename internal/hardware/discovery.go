// Package hardware discovers printers registered with the Windows spooler
// so the settings UI can offer them as dropdown choices.
package hardware

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ListPrinters returns the names of all printers currently installed on
// this machine. Get-Printer is preferred, with Win32_Printer as a fallback
// for systems where the PrintManagement module is unavailable.
func ListPrinters() ([]string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"$ErrorActionPreference='Stop'; $names=@(); try {$names=@(Get-Printer -ErrorAction Stop | Select-Object -ExpandProperty Name)} catch {}; if ($names.Count -eq 0) {$names=@(Get-CimInstance -ClassName Win32_Printer -ErrorAction Stop | Select-Object -ExpandProperty Name)}; $names",
	)
	setHiddenAttrs(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("enumerate printers: %w (stderr: %s)", err, stderr.String())
	}

	var names []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}
