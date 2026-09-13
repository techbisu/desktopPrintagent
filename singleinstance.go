//go:build windows

package main

import (
	"os"

	"github.com/lxn/walk"
	"golang.org/x/sys/windows"
)

// mutexName must be unique to this application. Using "Global\" makes the
// lock apply across all user sessions on the machine, not just the current
// user's session.
const mutexName = `Global\SmartPrintAgent-9f1c9c1e-9a3e-4e2a-8e5e-2e9f0d1a4c3b`

// ensureSingleInstance exits the process immediately if another copy of
// the agent is already running. Windows automatically releases the mutex
// when the owning process exits, so no explicit cleanup is needed.
func ensureSingleInstance() {
	namePtr, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		// Extremely unlikely; fail open rather than block startup entirely.
		return
	}

	_, err = windows.CreateMutex(nil, false, namePtr)
	if err == windows.ERROR_ALREADY_EXISTS {
		walk.MsgBox(nil, "SmartPrint Agent",
			"SmartPrint Agent is already running. Check your system tray.",
			walk.MsgBoxIconInformation)
		os.Exit(0)
	}
}
