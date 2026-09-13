//go:build windows

package main

import "log"

func main() {
	ensureSingleInstance()

	app := NewApp()
	app.Start()
	defer app.Stop()

	if err := runUI(app); err != nil {
		log.Fatalf("failed to start UI: %v", err)
	}
}
