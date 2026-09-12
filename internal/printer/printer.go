// Package printer wraps the embedded SumatraPDF binary to perform silent,
// headless printing on Windows without ever stealing focus from the
// shopkeeper's active window.
package printer

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed bin/SumatraPDF.exe
var sumatraBinary []byte

const (
	// CREATE_NO_WINDOW prevents a console window from flashing up.
	createNoWindow = 0x08000000
)

// Job describes everything ExecutePrint needs to send a file to a printer.
type Job struct {
	FilePath    string
	PrinterName string
	Copies      int
	Color       bool
	Duplex      bool
}

// Engine owns the extracted SumatraPDF binary path and guarantees it is
// only ever extracted once per process, even under concurrent workers.
type Engine struct {
	once    sync.Once
	binPath string
	initErr error
}

// NewEngine returns an Engine ready to have Ensure called on it.
func NewEngine() *Engine {
	return &Engine{}
}

// Ensure extracts the embedded SumatraPDF binary to
// %LocalAppData%/SmartPrint/bin/SumatraPDF.exe the first time it's called.
func (e *Engine) Ensure() error {
	e.once.Do(func() {
		localAppData, err := os.UserCacheDir()
		if err != nil {
			e.initErr = fmt.Errorf("resolve local app data dir: %w", err)
			return
		}
		binDir := filepath.Join(localAppData, "SmartPrint", "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			e.initErr = fmt.Errorf("create bin dir: %w", err)
			return
		}

		binPath := filepath.Join(binDir, "SumatraPDF.exe")
		if info, err := os.Stat(binPath); err != nil || info.Size() != int64(len(sumatraBinary)) {
			if err := os.WriteFile(binPath, sumatraBinary, 0o755); err != nil {
				e.initErr = fmt.Errorf("write embedded binary: %w", err)
				return
			}
		}
		e.binPath = binPath
	})
	return e.initErr
}

// ExecutePrint shells out to SumatraPDF with the correct print-settings
// string for the requested copies / color / duplex combination, hiding the
// console window and never activating it.
func (e *Engine) ExecutePrint(job Job) error {
	if err := e.Ensure(); err != nil {
		return err
	}
	if job.PrinterName == "" {
		return fmt.Errorf("no printer assigned for this job")
	}
	if job.Copies < 1 {
		job.Copies = 1
	}

	colorSetting := "monochrome"
	if job.Color {
		colorSetting = "color"
	}
	duplexSetting := "simplex"
	if job.Duplex {
		duplexSetting = "duplex"
	}

	printSettings := fmt.Sprintf("%dx,%s,%s,noscale", job.Copies, colorSetting, duplexSetting)

	cmd := exec.Command(
		e.binPath,
		"-print-to", job.PrinterName,
		"-print-settings", printSettings,
		"-silent",
		job.FilePath,
	)
	setHiddenAttrs(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sumatra print failed: %w (output: %s)", err, string(output))
	}
	return nil
}
