package main

import (
	"context"
	"embed"
	"log"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIcon []byte

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "SmartPrint Agent",
		Width:            850,
		Height:           580,
		MinWidth:         850,
		MinHeight:        580,
		DisableResize:    false,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "smartprint-agent-9f1c9c1e-9a3e-4e2a-8e5e-2e9f0d1a4c3b",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				// A shopkeeper double-clicked the shortcut again; bring the
				// existing window to the front instead of opening a second
				// copy of the agent.
				if app.ctx != nil {
					wailsRuntime.WindowShow(app.ctx)
					wailsRuntime.WindowUnminimise(app.ctx)
				}
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Assets:     assets,
		OnStartup:  app.OnStartup,
		OnShutdown: app.OnShutdown,
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			// Minimize to tray instead of exiting: the agent needs to keep
			// listening for print jobs even when the shopkeeper closes the
			// window.
			wailsRuntime.WindowHide(ctx)
			return true
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// startTray sets up the system tray icon and menu. Call this once from
// App.OnStartup (see app.go) — it runs its own event loop in a goroutine
// and is safe to call alongside Wails' own message loop on Windows.
func startTray(ctx context.Context, iconBytes []byte) {
	go systray.Run(func() {
		systray.SetIcon(iconBytes)
		systray.SetTitle("SmartPrint Agent")
		systray.SetTooltip("SmartPrint Agent — listening for print jobs")

		showItem := systray.AddMenuItem("Show Window", "Show the SmartPrint Agent window")
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("Quit", "Quit SmartPrint Agent")

		go func() {
			for {
				select {
				case <-showItem.ClickedCh:
					wailsRuntime.WindowShow(ctx)
					wailsRuntime.WindowUnminimise(ctx)
				case <-quitItem.ClickedCh:
					systray.Quit()
					wailsRuntime.Quit(ctx)
					return
				}
			}
		}()
	}, func() {
		// onExit: nothing to clean up beyond what OnShutdown already does.
	})
}
