//go:build windows

package main

import (
	_ "embed"
	"log"
	"os"
	"path/filepath"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

//go:embed build/windows/icon.ico
var appIconBytes []byte

// setupTray creates the system tray icon with a Show/Quit menu, and wires
// a left-click to bring the window back. quitting must be set to true
// before calling mw.Close() from the Quit action, so the window's Closing
// handler knows to allow the real close instead of hiding to tray again.
func setupTray(mw *walk.MainWindow, quitting *bool) (*walk.NotifyIcon, error) {
	icon, err := loadAppIcon()
	if err != nil {
		log.Printf("tray icon load failed, using default: %v", err)
		icon = nil
	}

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		return nil, err
	}

	if icon != nil {
		if err := ni.SetIcon(icon); err != nil {
			log.Printf("failed to set tray icon: %v", err)
		}
	}
	if err := ni.SetToolTip("SmartPrint Agent — listening for print jobs"); err != nil {
		log.Printf("failed to set tray tooltip: %v", err)
	}
	if err := ni.SetVisible(true); err != nil {
		return nil, err
	}

	showAction := walk.NewAction()
	showAction.SetText("Show window")
	showAction.Triggered().Attach(func() {
		mw.Show()
		mw.SetFocus()
		win32Restore(mw)
	})
	if err := ni.ContextMenu().Actions().Add(showAction); err != nil {
		log.Printf("failed to add tray show action: %v", err)
	}

	quitAction := walk.NewAction()
	quitAction.SetText("Quit")
	quitAction.Triggered().Attach(func() {
		*quitting = true
		mw.Close()
	})
	if err := ni.ContextMenu().Actions().Add(quitAction); err != nil {
		log.Printf("failed to add tray quit action: %v", err)
	}

	// Left-click on the tray icon also shows the window — the menu above
	// only appears on right-click by default.
	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			mw.Show()
			mw.SetFocus()
			win32Restore(mw)
		}
	})

	return ni, nil
}

// loadAppIcon writes the embedded icon bytes to a temp file and loads it,
// since walk's icon loader needs a file path rather than an in-memory
// buffer for .ico resources.
func loadAppIcon() (*walk.Icon, error) {
	dir := filepath.Join(os.TempDir(), "SmartPrint")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "icon.ico")
	if err := os.WriteFile(path, appIconBytes, 0o600); err != nil {
		return nil, err
	}
	return walk.NewIconFromFile(path)
}

// win32Restore un-minimizes the window if it was minimized before being
// hidden to the tray.
func win32Restore(mw *walk.MainWindow) {
	win.ShowWindow(mw.Handle(), win.SW_RESTORE)
}
