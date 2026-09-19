@echo off
setlocal
echo ===================================================
echo           SmartPrint Agent Uninstaller
echo ===================================================
echo.

:: 1. Force kill any running instances immediately
echo [1/3] Stopping any running background instances...
taskkill /F /IM SmartPrintAgent.exe /IM SumatraPDF.exe /T >nul 2>&1

:: 2. Check if installed via Inno Setup (in Program Files)
if exist "%ProgramFiles%\SmartPrint Agent\unins000.exe" (
    echo [2/3] Launching official Windows Uninstaller...
    "%ProgramFiles%\SmartPrint Agent\unins000.exe"
    goto :cleanup
)

:: 3. Fallback to PowerShell uninstaller script
if exist "%~dp0install.ps1" (
    echo [2/3] Running PowerShell uninstall routine...
    powershell.exe -ExecutionPolicy Bypass -Command "Start-Process powershell.exe -ArgumentList '-ExecutionPolicy Bypass -NoProfile -File \"\"%~dp0install.ps1\"\" -Uninstall' -Verb RunAs -Wait"
    goto :done
)

:cleanup
echo [3/3] Ensuring local temporary files and registry run keys are cleaned...
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v "SmartPrintAgent" /f >nul 2>&1
if exist "%LOCALAPPDATA%\SmartPrint" rd /s /q "%LOCALAPPDATA%\SmartPrint" >nul 2>&1
if exist "%APPDATA%\SmartPrint" rd /s /q "%APPDATA%\SmartPrint" >nul 2>&1
if exist "%TEMP%\SmartPrint" rd /s /q "%TEMP%\SmartPrint" >nul 2>&1

:done
echo.
echo [OK] Uninstallation routine finished.
exit /b 0
