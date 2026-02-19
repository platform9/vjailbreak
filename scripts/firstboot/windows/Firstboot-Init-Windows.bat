@echo off
setlocal EnableDelayedExpansion

:: ────────────────────────────────────────────────
::  Configuration
:: ────────────────────────────────────────────────
set "PS_SCRIPT=C:\firstboot\0-Firstboot-Scheduler.ps1"
set "LOGDIR=C:\firstboot"
set "LOGFILE=%LOGDIR%\Firstboot-Scheduler_%DATE:~-4%%DATE:~3,2%%DATE:~0,2%_%TIME:~0,2%%TIME:~3,2%.log"

:: Replace space in time with zero if hour < 10
set "LOGFILE=%LOGFILE: =0%"

:: ────────────────────────────────────────────────
::  Create log directory if missing
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
::  Header in log
:: ────────────────────────────────────────────────
echo [%DATE% %TIME%] ============================================== >> "%LOGFILE%"
echo [%DATE% %TIME%] Starting Firstboot Scheduler                  >> "%LOGFILE%"
echo [%DATE% %TIME%] Script: %PS_SCRIPT%                           >> "%LOGFILE%"
echo [%DATE% %TIME%] Computer: %COMPUTERNAME%                      >> "%LOGFILE%"
echo [%DATE% %TIME%] User:     %USERNAME%                          >> "%LOGFILE%"
echo [%DATE% %TIME%] ============================================== >> "%LOGFILE%"

:: ────────────────────────────────────────────────
::  Check if PowerShell script exists
:: ────────────────────────────────────────────────
if not exist "%PS_SCRIPT%" (
    echo [%DATE% %TIME%] ERROR: PowerShell script not found at:     >> "%LOGFILE%"
    echo [%DATE% %TIME%]        %PS_SCRIPT%                         >> "%LOGFILE%"
    echo.
    echo ERROR: Script not found: %PS_SCRIPT%
    echo        Check path and try again.
    echo.
    pause
    exit /b 1
)

:: ────────────────────────────────────────────────
::  Self-elevate to Administrator if not already
:: ────────────────────────────────────────────────
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo [%DATE% %TIME%] Requesting administrator rights...         >> "%LOGFILE%"
    echo.
    echo Requesting admin rights ─ please accept the UAC prompt...
    echo.

    :: Attempt to elevate via UAC
    powershell -NoProfile -ExecutionPolicy Bypass -Command ^
        "Start-Process cmd -ArgumentList '/c %~f0' -Verb RunAs -Wait" 2>nul

    :: Check if elevation succeeded
    if %errorlevel% neq 0 (
        echo [%DATE% %TIME%] UAC elevation failed or was denied.    >> "%LOGFILE%"
        echo [%DATE% %TIME%] Attempting to schedule for next boot using PowerShell... >> "%LOGFILE%"
        echo.
        echo UAC elevation failed or was denied.
        echo Attempting to schedule the script to run at next boot...
        echo.

        :: Create a scheduled task using PowerShell to run at system startup
        powershell -NoProfile -ExecutionPolicy Bypass -Command ^
            "$Action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -ExecutionPolicy Bypass -File \"%PS_SCRIPT%\"'; " ^
            "$Trigger = New-ScheduledTaskTrigger -AtStartup; " ^
            "$Principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -RunLevel Highest; " ^
            "$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries; " ^
            "Register-ScheduledTask -TaskName 'FirstbootScheduler-Elevated' -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -Force; " ^
            "exit $LASTEXITCODE" >nul 2>&1

        if %errorlevel% equ 0 (
            echo [%DATE% %TIME%] Scheduled task created successfully via PowerShell. >> "%LOGFILE%"
            echo.
            echo ============================================================
            echo  Firstboot Scheduler has been scheduled to run at next
            echo  system startup with SYSTEM privileges.
            echo.
            echo  Please restart the computer.
            echo  The script will run automatically during boot.
            echo ============================================================
            echo.
            echo [%DATE% %TIME%] Exiting - will run at next startup.      >> "%LOGFILE%"
            pause
            exit /b 0
        ) else (
            echo [%DATE% %TIME%] ERROR: Failed to create scheduled task via PowerShell. >> "%LOGFILE%"
            echo.
            echo ============================================================
            echo  ERROR: Could not obtain administrator rights.
            echo.
            echo  The script could not be scheduled for automatic
            echo  execution. Please run this script as an administrator
            echo  manually or contact your system administrator.
            echo ============================================================
            echo.
            echo [%DATE% %TIME%] All elevation methods failed.            >> "%LOGFILE%"
            pause
            exit /b 1
        )
    )

    exit /b
)

:: ────────────────────────────────────────────────
::  Now we are elevated ─ run the real PowerShell script
:: ────────────────────────────────────────────────
echo [%DATE% %TIME%] Running PowerShell script as Administrator...  >> "%LOGFILE%"
echo.                                                            >> "%LOGFILE%"

powershell.exe -NoProfile -ExecutionPolicy Bypass ^
    -Command "& '%PS_SCRIPT%' *>> '%LOGFILE%' 2>&1"

set PS_EXITCODE=%errorlevel%

echo.                                                            >> "%LOGFILE%"
echo [%DATE% %TIME%] PowerShell script finished.                  >> "%LOGFILE%"
echo [%DATE% %TIME%] Exit code: !PS_EXITCODE!                     >> "%LOGFILE%"

if !PS_EXITCODE! equ 0 (
    echo [%DATE% %TIME%] Result: SUCCESS                              >> "%LOGFILE%"
    echo.
    echo Firstboot Scheduler completed successfully.
    echo Log saved to:
    echo   %LOGFILE%
) else (
    echo [%DATE% %TIME%] Result: FAILED (exit code !PS_EXITCODE!)     >> "%LOGFILE%"
    echo.
    echo Firstboot Scheduler FAILED (exit code !PS_EXITCODE!).
    echo Check the log for details:
    echo   %LOGFILE%
)

echo.
echo [%DATE% %TIME%] Finished. Press any key to exit...           >> "%LOGFILE%"
pause >nul
exit /b !PS_EXITCODE!