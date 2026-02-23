@echo off
setlocal EnableDelayedExpansion

:: ────────────────────────────────────────────────
:: Configuration
:: ────────────────────────────────────────────────
set "PS_SCRIPT=C:\firstboot\0-Firstboot-Scheduler.ps1"
set "LOGDIR=C:\firstboot"
set "MARKERFILE=%LOGDIR%\Firstboot-Completed.marker"
set "TASKNAME=FirstbootScheduler-Elevated"

:: Build timestamp-safe log name (YYYYMMDD_HHMM)
for /f "tokens=2 delims==" %%a in ('wmic OS Get localdatetime /value') do set "dt=%%a"
set "LOGFILE=%LOGDIR%\Firstboot-Scheduler_%dt:~0,8%_%dt:~8,4%.log"

:: ────────────────────────────────────────────────
:: Create log directory if missing
:: ────────────────────────────────────────────────
if not exist "%LOGDIR%\" (
    mkdir "%LOGDIR%" 2>nul
    if errorlevel 1 (
        echo ERROR: Cannot create log directory %LOGDIR%
        pause
        exit /b 1
    )
)

:: ────────────────────────────────────────────────
:: Header in log
:: ────────────────────────────────────────────────
>>"%LOGFILE%" echo [%DATE% %TIME%] ==============================================
>>"%LOGFILE%" echo [%DATE% %TIME%] Starting Firstboot Scheduler Launcher
>>"%LOGFILE%" echo [%DATE% %TIME%] Target script: %PS_SCRIPT%
>>"%LOGFILE%" echo [%DATE% %TIME%] Computer: %COMPUTERNAME%
>>"%LOGFILE%" echo [%DATE% %TIME%] User: %USERNAME%
>>"%LOGFILE%" echo [%DATE% %TIME%] ==============================================

:: ────────────────────────────────────────────────
:: Check if already completed (optional - remove if unwanted)
:: ────────────────────────────────────────────────
if exist "%MARKERFILE%" (
    >>"%LOGFILE%" echo [%DATE% %TIME%] Marker file exists - assuming firstboot already completed.
    echo Firstboot appears to have already run successfully.
    echo (Marker: %MARKERFILE%)
    pause
    exit /b 0
)

:: ────────────────────────────────────────────────
:: Check if PowerShell script exists
:: ────────────────────────────────────────────────
if not exist "%PS_SCRIPT%" (
    >>"%LOGFILE%" echo [%DATE% %TIME%] ERROR: PowerShell script not found: %PS_SCRIPT%
    echo.
    echo ERROR: Script not found: %PS_SCRIPT%
    echo Check path and try again.
    echo.
    pause
    exit /b 1
)

:: ────────────────────────────────────────────────
:: Check if already running elevated
:: ────────────────────────────────────────────────
net session >nul 2>&1
if %errorlevel% equ 0 goto :RUN_ELEVATED

:: ────────────────────────────────────────────────
:: Not elevated → try UAC elevation first
:: ────────────────────────────────────────────────
>>"%LOGFILE%" echo [%DATE% %TIME%] Not running as Administrator - requesting elevation...
echo.
echo Requesting administrator rights...
echo Please accept the UAC prompt if it appears...
echo.

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "Start-Process cmd -ArgumentList '/c \"%~f0\"' -Verb RunAs -Wait" 2>nul

:: If we reach here → elevation was denied or failed
>>"%LOGFILE%" echo [%DATE% %TIME%] UAC elevation failed or was denied.

:: ────────────────────────────────────────────────
:: Fallback: Schedule task to run at next boot as SYSTEM
:: ────────────────────────────────────────────────
>>"%LOGFILE%" echo [%DATE% %TIME%] Attempting to create startup scheduled task as SYSTEM...

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "$ErrorActionPreference = 'Stop'; " ^
    "try { " ^
    "  $Action   = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -ExecutionPolicy Bypass -File \"%PS_SCRIPT%\"'; " ^
    "  $Trigger  = New-ScheduledTaskTrigger -AtStartup; " ^
    "  $Principal = New-ScheduledTaskPrincipal -UserId 'NT AUTHORITY\SYSTEM' -LogonType ServiceAccount -RunLevel Highest; " ^
    "  $Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -Hidden -StartWhenAvailable; " ^
    "  Register-ScheduledTask -TaskName '%TASKNAME%' -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -Force -ErrorAction Stop; " ^
    "  Write-Output 'Scheduled task created successfully'; " ^
    "  exit 0 " ^
    "} catch { " ^
    "  Write-Error $_.Exception.Message; " ^
    "  exit 1 " ^
    "}"  >>"%LOGFILE%" 2>&1

if %errorlevel% equ 0 (
    >>"%LOGFILE%" echo [%DATE% %TIME%] SUCCESS: Scheduled task '%TASKNAME%' created.
    echo.
    echo ============================================================
    echo   Firstboot has been SCHEDULED to run at next system startup
    echo   (as SYSTEM with highest privileges)
    echo.
    echo   Please restart the computer now.
    echo   The script %PS_SCRIPT% will run automatically during boot.
    echo ============================================================
    echo.
    pause
    exit /b 0
) else (
    >>"%LOGFILE%" echo [%DATE% %TIME%] ERROR: Failed to create scheduled task (exit code %errorlevel%).
    echo.
    echo ============================================================
    echo   CRITICAL ERROR - Could not obtain administrator rights
    echo   and could not schedule automatic execution.
    echo.
    echo   Please:
    echo     1. Right-click this file → Run as administrator
    echo     2. Or open Task Scheduler manually and create a task
    echo        named '%TASKNAME%' to run %PS_SCRIPT% at startup
    echo   Log saved to: %LOGFILE%
    echo ============================================================
    echo.
    pause
    exit /b 1
)

:RUN_ELEVATED
:: ────────────────────────────────────────────────
:: We are now running elevated ─ execute the real script
:: ────────────────────────────────────────────────
>>"%LOGFILE%" echo [%DATE% %TIME%] Running elevated: %PS_SCRIPT%
>>"%LOGFILE%" echo.

powershell.exe -NoProfile -ExecutionPolicy Bypass ^
    -Command "& '%PS_SCRIPT%' *>> '%LOGFILE%' 2>&1"

set "PS_EXITCODE=%errorlevel%"

>>"%LOGFILE%" echo.
>>"%LOGFILE%" echo [%DATE% %TIME%] PowerShell script finished with exit code %PS_EXITCODE%

if %PS_EXITCODE% equ 0 (
    >>"%LOGFILE%" echo [%DATE% %TIME%] Result: SUCCESS
    echo.
    echo Firstboot Scheduler completed successfully.
    echo Log: %LOGFILE%
    :: Optional: create completion marker
    echo Completed at %DATE% %TIME% > "%MARKERFILE%"
) else (
    >>"%LOGFILE%" echo [%DATE% %TIME%] Result: FAILED (code %PS_EXITCODE%)
    echo.
    echo Firstboot Scheduler FAILED (exit code %PS_EXITCODE%).
    echo Please check the log:
    echo %LOGFILE%
)

echo.
>>"%LOGFILE%" echo [%DATE% %TIME%] Launcher finished.
pause >nul
exit /b %PS_EXITCODE%