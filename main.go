//go:build windows

package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/lxn/walk"
)

var logFile *os.File

// configureLogging preserves startup errors that would otherwise be hidden
// because production builds use the Windows GUI subsystem and have no console.
func configureLogging() {
	dir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	dir = filepath.Join(dir, "SmartPrint")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	logFile, err = os.OpenFile(filepath.Join(dir, "agent.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
}

func main() {
	configureLogging()
	defer func() {
		if logFile != nil {
			_ = logFile.Close()
		}
	}()
	log.Printf("SmartPrint Agent starting")
	ensureSingleInstance()

	startMinimized := false
	for _, arg := range os.Args[1:] {
		if arg == "-minimized" || arg == "--minimized" || arg == "/minimized" {
			startMinimized = true
			break
		}
	}

	app := NewApp()
	app.Start()
	defer app.Stop()

	if err := runUI(app, startMinimized); err != nil {
		log.Printf("failed to start UI: %v", err)
		walk.MsgBox(nil, "SmartPrint Agent", "The application could not start. See %LocalAppData%\\SmartPrint\\agent.log for details.\n\n"+err.Error(), walk.MsgBoxIconError)
	}
}
