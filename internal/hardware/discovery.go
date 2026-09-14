// Package hardware discovers printers registered with the Windows spooler
// so the settings UI can offer them as dropdown choices.
package hardware

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// PrinterState describes what Windows currently reports for one installed
// printer. A successful silent-print process only means that Windows accepted
// the job; this state lets the queue warn before handing a job to an offline
// spooler.
type PrinterState struct {
	Found   bool
	Offline bool
	Status  string
}

// ListPrinters returns the names of all printers currently installed on
// this machine. Get-Printer is preferred, with the older Win32_Printer WMI
// provider as a fallback for Windows 7 and systems without PrintManagement.
func ListPrinters() ([]string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"$ErrorActionPreference='Stop'; $names=@(); try {$names=@(Get-Printer -ErrorAction Stop | Select-Object -ExpandProperty Name)} catch {}; if ($names.Count -eq 0) {$names=@(Get-WmiObject -Class Win32_Printer -ErrorAction Stop | Select-Object -ExpandProperty Name)}; $names",
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

// GetPrinterState returns the spooler's current view of a named printer.
// It intentionally supports both the modern Get-Printer cmdlet and the WMI
// provider present on Windows 7/8.
func GetPrinterState(name string) (PrinterState, error) {
	if strings.TrimSpace(name) == "" {
		return PrinterState{}, fmt.Errorf("no printer selected")
	}

	script := "$ErrorActionPreference='Stop'; $name=[Environment]::GetEnvironmentVariable('SMARTPRINT_PRINTER_NAME','Process'); $p=$null; try {$p=Get-Printer -Name $name -ErrorAction Stop} catch {}; if ($null -eq $p) {$p=Get-WmiObject -Class Win32_Printer -ErrorAction Stop | Where-Object {$_.Name -eq $name} | Select-Object -First 1}; if ($null -eq $p) {'NOT_FOUND'; exit 0}; $offline=[bool]$p.WorkOffline; $status=[string]$p.PrinterStatus; if ($offline -or $status -match 'Offline|Error|Not Available') {'OFFLINE|' + $status} else {'ONLINE|' + $status}"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(cmd.Environ(), "SMARTPRINT_PRINTER_NAME="+name)
	setHiddenAttrs(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return PrinterState{}, fmt.Errorf("check printer state: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 2)
	if len(parts) == 0 || parts[0] == "" {
		return PrinterState{}, fmt.Errorf("check printer state: no response")
	}
	if parts[0] == "NOT_FOUND" {
		return PrinterState{Status: "Not installed"}, nil
	}
	state := PrinterState{Found: true}
	if len(parts) == 2 {
		state.Status = parts[1]
	}
	state.Offline = parts[0] == "OFFLINE"
	return state, nil
}
