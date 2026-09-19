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
	if err != nil || icon == nil {
		log.Printf("tray icon load failed, using application fallback: %v", err)
		icon = walk.IconApplication()
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
	showAction.SetText("Open SmartPrint Agent")
	showAction.Triggered().Attach(func() {
		mw.Show()
		mw.SetFocus()
		win32Restore(mw)
	})
	if err := ni.ContextMenu().Actions().Add(showAction); err != nil {
		log.Printf("failed to add tray show action: %v", err)
	}

	if sep, err := walk.NewSeparatorAction(); err == nil {
		_ = ni.ContextMenu().Actions().Add(sep)
	}

	quitAction := walk.NewAction()
	quitAction.SetText("Quit SmartPrint Agent")
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
	if len(appIconBytes) == 0 {
		return walk.IconApplication(), nil
	}
	dir := filepath.Join(os.TempDir(), "SmartPrint")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return walk.IconApplication(), nil
	}
	path := filepath.Join(dir, "icon.ico")
	if err := os.WriteFile(path, appIconBytes, 0o600); err != nil {
		return walk.IconApplication(), nil
	}
	icon, err := walk.NewIconFromFile(path)
	if err != nil {
		return walk.IconApplication(), nil
	}
	return icon, nil
}

// win32Restore un-minimizes the window if it was minimized before being
// hidden to the tray.
func win32Restore(mw *walk.MainWindow) {
	win.ShowWindow(mw.Handle(), win.SW_RESTORE)
}
