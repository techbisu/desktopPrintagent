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
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-kill", "--kill", "/kill", "-quit", "--quit", "/quit":
			log.Printf("Kill/quit argument detected, stopping all running instances...")
			KillAllInstances()
			return
		case "-uninstall", "--uninstall", "/uninstall":
			log.Printf("Uninstall argument detected...")
			_ = SetAutoStart(false)
			KillAllInstances()
			tempDir := filepath.Join(os.TempDir(), "SmartPrint")
			_ = os.RemoveAll(tempDir)
			if cacheDir, err := os.UserCacheDir(); err == nil {
				_ = os.RemoveAll(filepath.Join(cacheDir, "SmartPrint"))
			}
			return
		case "-minimized", "--minimized", "/minimized":
			startMinimized = true
		}
	}

	ensureSingleInstance()

	app := NewApp()
	app.Start()
	defer app.Stop()

	if err := runUI(app, startMinimized); err != nil {
		log.Printf("failed to start UI: %v", err)
		walk.MsgBox(nil, "SmartPrint Agent", "The application could not start. See %LocalAppData%\\SmartPrint\\agent.log for details.\n\n"+err.Error(), walk.MsgBoxIconError)
	}
}
