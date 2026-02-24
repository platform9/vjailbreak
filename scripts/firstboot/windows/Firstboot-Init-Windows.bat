@echo off
setlocal EnableDelayedExpansion

:: ────────────────────────────────────────────────
:: Configuration
:: ────────────────────────────────────────────────
set "PS_SCRIPT=C:\firstboot\0-Firstboot-Scheduler.ps1"
set "LOGDIR=C:\firstboot"
set "TASK_NAME=Firstboot-Scheduler-Auto"

:: Create timestamp for log (YYYYMMDD_HHMM)
for /f "tokens=2 delims==" %%a in ('wmic OS Get localdatetime /value') do set "dt=%%a"
set "LOGFILE=%LOGDIR%\Firstboot-Scheduler_%dt:~0,4%%dt:~4,2%%dt:~6,2%_%dt:~8,2%%dt:~10,2%.log"

:: ────────────────────────────────────────────────
:: Create log directory if it doesn't exist
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
:: Write header to log
:: ────────────────────────────────────────────────
>> "%LOGFILE%" echo [%DATE% %TIME%] ==============================================
>> "%LOGFILE%" echo [%DATE% %TIME%] Starting Firstboot Scheduler
>> "%LOGFILE%" echo [%DATE% %TIME%] Target script: %PS_SCRIPT%
>> "%LOGFILE%" echo [%DATE% %TIME%] Computer: %COMPUTERNAME%
>> "%LOGFILE%" echo [%DATE% %TIME%] User: %USERNAME%
>> "%LOGFILE%" echo [%DATE% %TIME%] ==============================================

:: ────────────────────────────────────────────────
:: Check if the PowerShell script exists
:: ────────────────────────────────────────────────
if not exist "%PS_SCRIPT%" (
    >> "%LOGFILE%" echo [%DATE% %TIME%] ERROR: PowerShell script not found: %PS_SCRIPT%
    echo.
    echo ERROR: Target script not found:
    echo %PS_SCRIPT%
    echo.
    pause
    exit /b 1
)

:: ────────────────────────────────────────────────
:: Check if already running elevated
:: ────────────────────────────────────────────────
net session >nul 2>&1
if %errorlevel% equ 0 (
    goto :RUN_POWERSHELL
)

:: ────────────────────────────────────────────────
:: Not elevated → create one-time startup task as SYSTEM
:: ────────────────────────────────────────────────
>> "%LOGFILE%" echo [%DATE% %TIME%] Not running elevated. Creating startup task...

schtasks /Create /TN "%TASK_NAME%" ^
    /TR "powershell.exe -NoProfile -ExecutionPolicy Bypass -File \"%PS_SCRIPT%\" *>> \"%LOGFILE%\" 2>&1" ^
    /SC ONSTART /RU SYSTEM /RL HIGHEST /F >nul 2>&1

if %errorlevel% equ 0 (
    >> "%LOGFILE%" echo [%DATE% %TIME%] Scheduled task '%TASK_NAME%' created successfully.
    >> "%LOGFILE%" echo.
    >> "%LOGFILE%" echo ============================================================
    >> "%LOGFILE%" echo The script is now scheduled to run AUTOMATICALLY at next system startup
    >> "%LOGFILE%" echo as SYSTEM (no UAC prompt will appear).
    >> "%LOGFILE%" echo.
    >> "%LOGFILE%" echo Please RESTART the computer now.
    >> "%LOGFILE%" echo After successful execution, the task will be automatically deleted.
    >> "%LOGFILE%" echo ============================================================
    echo.
    echo ============================================================
    echo Scheduled to run at next boot as SYSTEM (no prompt).
    echo Please restart the computer now.
    echo ============================================================
    echo.
    pause
    exit /b 0
) else (
    >> "%LOGFILE%" echo [%DATE% %TIME%] ERROR: Failed to create scheduled task (errorlevel %errorlevel%).
    >> "%LOGFILE%" echo Most common cause: current user is not a local Administrator.
    echo.
    echo ============================================================
    echo ERROR: Could not create startup task
    echo.
    echo This script must be run **once** as Administrator
    echo so it can create the automatic startup task.
    echo After that first run, it will execute silently at boot.
    echo ============================================================
    echo.
    pause
    exit /b 1
)

:RUN_POWERSHELL
:: ────────────────────────────────────────────────
:: We are now running elevated (admin or SYSTEM)
:: ────────────────────────────────────────────────
>> "%LOGFILE%" echo [%DATE% %TIME%] Running elevated ─ executing main PowerShell script...
>> "%LOGFILE%" echo.

powershell.exe -NoProfile -ExecutionPolicy Bypass ^
    -Command "& '%PS_SCRIPT%' *>> '%LOGFILE%' 2>&1"

set "PS_EXITCODE=%errorlevel%"

>> "%LOGFILE%" echo.
>> "%LOGFILE%" echo [%DATE% %TIME%] PowerShell script finished.
>> "%LOGFILE%" echo [%DATE% %TIME%] Exit code: %PS_EXITCODE%

if %PS_EXITCODE% equ 0 (
    >> "%LOGFILE%" echo [%DATE% %TIME%] Result: SUCCESS
    echo.
    echo Firstboot Scheduler completed successfully.
    echo Log saved to:
    echo %LOGFILE%

    :: Optional: Clean up the task after successful run (one-time execution)
    schtasks /Delete /TN "%TASK_NAME%" /F >nul 2>&1
    if !errorlevel! equ 0 (
        >> "%LOGFILE%" echo [%DATE% %TIME%] Startup task '%TASK_NAME%' deleted (one-time run completed).
    ) else (
        >> "%LOGFILE%" echo [%DATE% %TIME%] Note: Could not delete task '%TASK_NAME%' (may already be gone).
    )
) else (
    >> "%LOGFILE%" echo [%DATE% %TIME%] Result: FAILED (exit code %PS_EXITCODE%)
    echo.
    echo Firstboot Scheduler FAILED (exit code %PS_EXITCODE%).
    echo Please check the log for details:
    echo %LOGFILE%
)

echo.
>> "%LOGFILE%" echo [%DATE% %TIME%] Finished. Press any key to exit...
pause >nul
exit /b %PS_EXITCODE%
