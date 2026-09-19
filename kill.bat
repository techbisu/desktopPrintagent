@echo off
echo ===================================================
echo   Stopping SmartPrint Agent and Printing Processes
echo ===================================================

taskkill /F /IM SmartPrintAgent.exe /IM SumatraPDF.exe /T >nul 2>&1

if %ERRORLEVEL% equ 0 (
    echo [OK] Successfully stopped SmartPrint Agent and helper processes.
) else (
    echo [INFO] No running SmartPrint Agent process was found.
)

echo.
exit /b 0
