<#
.SYNOPSIS
    SmartPrint Desktop Agent Installer Script
    Installs SmartPrint Agent to C:\Program Files\SmartPrint Agent,
    creates Start Menu & Desktop shortcuts, and sets up Auto-Start.

.NOTES
    Must be executed from an elevated (Administrator) PowerShell session.
#>

param (
    [switch]$Uninstall,
    [switch]$NoAutoStart
)

$ErrorActionPreference = "Stop"

$AppName = "SmartPrint Agent"
$InstallDir = "$env:ProgramFiles\SmartPrint Agent"
$ExeName = "SmartPrintAgent.exe"
$TargetPath = Join-Path $InstallDir $ExeName

# Require Administrator privileges
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Error "Please run this installation script from an elevated (Administrator) PowerShell prompt."
    exit 1
}

if ($Uninstall) {
    Write-Host "Uninstalling $AppName..." -ForegroundColor Yellow

    # Stop running process
    Get-Process -Name "SmartPrintAgent" -ErrorAction SilentlyContinue | Stop-Process -Force

    # Remove Registry Auto-Start
    $RunKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
    if (Get-ItemProperty -Path $RunKey -Name "SmartPrintAgent" -ErrorAction SilentlyContinue) {
        Remove-ItemProperty -Path $RunKey -Name "SmartPrintAgent" -Force
        Write-Host "Removed Auto-Start registry entry." -ForegroundColor Green
    }

    # Remove Shortcuts
    $StartMenuShortcut = "$env:ProgramData\Microsoft\Windows\Start Menu\Programs\$AppName.lnk"
    $DesktopShortcut = "$([Environment]::GetFolderPath('CommonDesktopDirectory'))\$AppName.lnk"
    if (Test-Path $StartMenuShortcut) { Remove-Item $StartMenuShortcut -Force }
    if (Test-Path $DesktopShortcut) { Remove-Item $DesktopShortcut -Force }

    # Remove Install Folder
    if (Test-Path $InstallDir) {
        Remove-Item -Path $InstallDir -Recurse -Force
        Write-Host "Removed $InstallDir." -ForegroundColor Green
    }

    Write-Host "$AppName has been completely uninstalled." -ForegroundColor Green
    exit 0
}

Write-Host "=============================================" -ForegroundColor Cyan
Write-Host "  Installing $AppName to C:\Program Files" -ForegroundColor Cyan
Write-Host "=============================================" -ForegroundColor Cyan

# Source files location (current directory or build/bin)
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$SourceExe = Join-Path $ScriptDir "SmartPrintAgent.exe"
$SourceManifest = Join-Path $ScriptDir "SmartPrintAgent.exe.manifest"
$SourceIcon = Join-Path $ScriptDir "build\windows\icon.ico"

if (-not (Test-Path $SourceExe)) {
    $SourceExe = Join-Path $ScriptDir "build\bin\SmartPrintAgent.exe"
    $SourceManifest = Join-Path $ScriptDir "build\bin\SmartPrintAgent.exe.manifest"
}

if (-not (Test-Path $SourceExe)) {
    Write-Error "Could not find SmartPrintAgent.exe in $ScriptDir or $ScriptDir\build\bin. Please build or download the executable first."
    exit 1
}

# Stop any running instances before overwriting
Get-Process -Name "SmartPrintAgent" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 500

# Create C:\Program Files\SmartPrint Agent
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Copy binary & assets
Copy-Item -Path $SourceExe -Destination $TargetPath -Force
Write-Host "[✓] Installed executable to $TargetPath" -ForegroundColor Green

if (Test-Path $SourceManifest) {
    Copy-Item -Path $SourceManifest -Destination (Join-Path $InstallDir "SmartPrintAgent.exe.manifest") -Force
}
if (Test-Path $SourceIcon) {
    Copy-Item -Path $SourceIcon -Destination (Join-Path $InstallDir "icon.ico") -Force
}

# Create Shortcuts via WScript.Shell
$WshShell = New-Object -ComObject WScript.Shell

# 1. Start Menu Shortcut
$StartMenuDir = "$env:ProgramData\Microsoft\Windows\Start Menu\Programs"
$StartShortcutPath = Join-Path $StartMenuDir "$AppName.lnk"
$Shortcut = $WshShell.CreateShortcut($StartShortcutPath)
$Shortcut.TargetPath = $TargetPath
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Description = "SmartPrint Background Print Agent"
if (Test-Path (Join-Path $InstallDir "icon.ico")) {
    $Shortcut.IconLocation = Join-Path $InstallDir "icon.ico"
}
$Shortcut.Save()
Write-Host "[✓] Created Start Menu shortcut" -ForegroundColor Green

# 2. Desktop Shortcut
$DesktopDir = [Environment]::GetFolderPath('CommonDesktopDirectory')
$DesktopShortcutPath = Join-Path $DesktopDir "$AppName.lnk"
$Shortcut = $WshShell.CreateShortcut($DesktopShortcutPath)
$Shortcut.TargetPath = $TargetPath
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Description = "SmartPrint Background Print Agent"
if (Test-Path (Join-Path $InstallDir "icon.ico")) {
    $Shortcut.IconLocation = Join-Path $InstallDir "icon.ico"
}
$Shortcut.Save()
Write-Host "[✓] Created Desktop shortcut" -ForegroundColor Green

# 3. Auto-Start Configuration (HKCU\...\Run)
if (-not $NoAutoStart) {
    $RunKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
    Set-ItemProperty -Path $RunKey -Name "SmartPrintAgent" -Value "`"$TargetPath`" -minimized" -Force
    Write-Host "[✓] Configured Auto-Start on Windows login (-minimized to tray)" -ForegroundColor Green
}

Write-Host "`nInstallation successfully completed!" -ForegroundColor Cyan
Write-Host "You can now launch SmartPrint Agent from your Start Menu, Desktop, or reboot to verify auto-start." -ForegroundColor White
