//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

const mutexName = `Global\SmartPrintAgent-9f1c9c1e-9a3e-4e2a-8e5e-2e9f0d1a4c3b`

// ensureSingleInstance ensures only one instance runs at a time.
// If another instance is already running, it attempts to bring its window to
// the foreground. If the existing instance is unresponsive or hidden,
// it prompts the user to terminate it and start fresh.
func ensureSingleInstance() {
	namePtr, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return
	}

	for attempt := 0; attempt < 2; attempt++ {
		_, err = windows.CreateMutex(nil, false, namePtr)
		if err == nil {
			return
		}

		if err == windows.ERROR_ALREADY_EXISTS {
			// First, try restoring an existing visible or minimized window
			if tryRestoreExistingWindow() {
				os.Exit(0)
			}

			// If no window is found (background/zombie process), offer to restart it
			msg := "Another instance of SmartPrint Agent is already running in the background.\n\n" +
				"Would you like to close the background instance and launch a fresh window?"
			if walk.MsgBox(nil, "SmartPrint Agent Already Running", msg, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes {
				killOtherInstances()
				time.Sleep(600 * time.Millisecond)
				continue
			}
			os.Exit(0)
		}
	}
}

func tryRestoreExistingWindow() bool {
	titlePtr, _ := windows.UTF16PtrFromString("SmartPrint Agent — Desktop Print Station")
	hwnd := win.FindWindow(nil, titlePtr)
	if hwnd != 0 {
		win.ShowWindow(hwnd, win.SW_RESTORE)
		win.SetForegroundWindow(hwnd)
		return true
	}
	return false
}

func killOtherInstances() {
	currentPID := os.Getpid()
	cmd := exec.Command("taskkill", "/F", "/FI", fmt.Sprintf("PID ne %d", currentPID), "/IM", "SmartPrintAgent.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
}
