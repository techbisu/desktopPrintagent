//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	runRegistryKey       = `Software\Microsoft\Windows\CurrentVersion\Run`
	appRegistryValueName = `SmartPrintAgent`
)

// IsAutoStartEnabled checks if SmartPrint Agent is configured to launch at Windows login.
func IsAutoStartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(appRegistryValueName)
	if err != nil || val == "" {
		return false
	}
	return true
}

// SetAutoStart configures or removes the HKCU Run registry key for SmartPrint Agent.
// When enabled, it registers the executable with the -minimized flag so it starts
// silently in the system tray upon Windows login.
func SetAutoStart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runRegistryKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if !enable {
		err := k.DeleteValue(appRegistryValueName)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return err
	}

	// Wrap in quotes and add -minimized flag
	cmd := `"` + exePath + `" -minimized`
	if err := k.SetStringValue(appRegistryValueName, cmd); err != nil {
		log.Printf("failed to set autostart registry value: %v", err)
		return err
	}
	return nil
}
