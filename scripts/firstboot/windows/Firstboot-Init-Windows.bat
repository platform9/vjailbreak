@echo off
setlocal EnableDelayedExpansion

:: ────────────────────────────────────────────────
::  Configuration
:: ────────────────────────────────────────────────
set "PS_SCRIPT=C:\firstboot\0-Firstboot-Scheduler.ps1"
set "LOGDIR=C:\firstboot"
set "LOGFILE=%LOGDIR%\Firstboot-Init_%DATE:~-4%%DATE:~3,2%%DATE:~0,2%_%TIME:~0,2%%TIME:~3,2%.log"

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
echo [%DATE% %TIME%] Starting Firstboot Initialization             >> "%LOGFILE%"
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
::  Check if 64-bit system
:: ────────────────────────────────────────────────
echo [%DATE% %TIME%] Checking system architecture...              >> "%LOGFILE%"
set IS_64BIT=0

:: Check if running on 64-bit OS
if "%PROCESSOR_ARCHITECTURE%"=="AMD64" set IS_64BIT=1
if "%PROCESSOR_ARCHITECTURE%"=="IA64" set IS_64BIT=1

:: For 32-bit process on 64-bit OS, check PROCESSOR_ARCHITEW6432
if defined PROCESSOR_ARCHITEW6432 set IS_64BIT=1

if %IS_64BIT% equ 1 (
    echo [%DATE% %TIME%] System is 64-bit                            >> "%LOGFILE%"
    echo System is 64-bit
) else (
    echo [%DATE% %TIME%] System is 32-bit                            >> "%LOGFILE%"
    echo System is 32-bit
)

:: ────────────────────────────────────────────────
::  Check for 64-bit PowerShell availability
:: ────────────────────────────────────────────────
echo [%DATE% %TIME%] Checking for 64-bit PowerShell...            >> "%LOGFILE%"
set "PS64_PATH=%SystemRoot%\sysnative\WindowsPowerShell\v1.0\powershell.exe"
set PS64_AVAILABLE=0

if %IS_64BIT% equ 1 (
    if exist "%PS64_PATH%" (
        set PS64_AVAILABLE=1
        echo [%DATE% %TIME%] 64-bit PowerShell found at: %PS64_PATH% >> "%LOGFILE%"
        echo 64-bit PowerShell found at: %PS64_PATH%
    ) else (
        echo [%DATE% %TIME%] 64-bit PowerShell not found              >> "%LOGFILE%"
        echo 64-bit PowerShell not found
    )
) else (
    echo [%DATE% %TIME%] 32-bit system - using default PowerShell   >> "%LOGFILE%"
    echo 32-bit system - using default PowerShell
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
    if %PS64_AVAILABLE% equ 1 (
        "%PS64_PATH%" -NoProfile -ExecutionPolicy Bypass -Command ^
            "Start-Process '%PS64_PATH%' -ArgumentList '-NoProfile -ExecutionPolicy Bypass -Command \"Start-Process cmd -ArgumentList \\\"/c %~f0\\\" -Verb RunAs -Wait\"' -Verb RunAs -Wait" 2>nul
    ) else (
        powershell -NoProfile -ExecutionPolicy Bypass -Command ^
            "Start-Process cmd -ArgumentList '/c %~f0' -Verb RunAs -Wait" 2>nul
    )

    :: Check if elevation succeeded
    if %errorlevel% neq 0 (
        echo [%DATE% %TIME%] UAC elevation failed or was denied.    >> "%LOGFILE%"
        echo [%DATE% %TIME%] Attempting to schedule for next boot using PowerShell scheduling mechanism... >> "%LOGFILE%"
        echo.
        echo UAC elevation failed or was denied.
        echo Attempting to schedule the script to run at next boot...
        echo.

        :: Create a scheduled task using PowerShell scheduling mechanism
        if %PS64_AVAILABLE% equ 1 (
            "%PS64_PATH%" -NoProfile -ExecutionPolicy Bypass -Command ^
                "$Action = New-ScheduledTaskAction -Execute '%PS64_PATH%' -Argument '-NoProfile -ExecutionPolicy Bypass -File \"%PS_SCRIPT%\"'; " ^
                "$Trigger = New-ScheduledTaskTrigger -AtStartup; " ^
                "$Principal = New-ScheduledTaskPrincipal -UserId 'NT AUTHORITY\SYSTEM' -LogonType ServiceAccount -RunLevel Highest; " ^
                "$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 30); " ^
                "if (Get-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -ErrorAction SilentlyContinue) { Unregister-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -Confirm:$false; Start-Sleep -Seconds 2 }; " ^
                "Register-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -Description 'Firstboot Scheduler' -Force; " ^
                "exit $LASTEXITCODE" >nul 2>&1
        ) else (
            powershell -NoProfile -ExecutionPolicy Bypass -Command ^
                "$Action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -ExecutionPolicy Bypass -File \"%PS_SCRIPT%\"'; " ^
                "$Trigger = New-ScheduledTaskTrigger -AtStartup; " ^
                "$Principal = New-ScheduledTaskPrincipal -UserId 'NT AUTHORITY\SYSTEM' -LogonType ServiceAccount -RunLevel Highest; " ^
                "$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 30); " ^
                "if (Get-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -ErrorAction SilentlyContinue) { Unregister-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -Confirm:$false; Start-Sleep -Seconds 2 }; " ^
                "Register-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -Description 'Firstboot Scheduler' -Force; " ^
                "exit $LASTEXITCODE" >nul 2>&1
        )

        if %errorlevel% equ 0 (
            echo [%DATE% %TIME%] Scheduled task created successfully using PowerShell scheduling mechanism. >> "%LOGFILE%"
            echo.
            echo ============================================================
            echo  Firstboot Scheduler has been scheduled to run at next
            echo  system startup with SYSTEM privileges using the proper
            echo  scheduling mechanism.
            echo.
            echo  Please restart the computer.
            echo  The script will run automatically during boot.
            echo ============================================================
            echo.
            echo [%DATE% %TIME%] Exiting - will run at next startup.      >> "%LOGFILE%"
            pause
            exit /b 0
        ) else (
            echo [%DATE% %TIME%] ERROR: Failed to create scheduled task via PowerShell scheduling mechanism. >> "%LOGFILE%"
            echo.
            echo ============================================================
            echo  ERROR: Could not obtain administrator rights and
            echo  failed to schedule the script.
            echo.
            echo  Please run this script as an administrator manually
            echo  or contact your system administrator.
            echo ============================================================
            echo.
            echo [%DATE% %TIME%] All elevation and scheduling methods failed. >> "%LOGFILE%"
            pause
            exit /b 1
        )
    )

    exit /b
)

:: ────────────────────────────────────────────────
::  Now we are elevated ─ run the PowerShell script
:: ────────────────────────────────────────────────
echo [%DATE% %TIME%] Running PowerShell script...                  >> "%LOGFILE%"
if %PS64_AVAILABLE% equ 1 (
    echo [%DATE% %TIME%] Using 64-bit PowerShell: %PS64_PATH%      >> "%LOGFILE%"
    echo Using 64-bit PowerShell: %PS64_PATH%
) else (
    echo [%DATE% %TIME%] Using default PowerShell                   >> "%LOGFILE%"
    echo Using default PowerShell
)
echo.                                                            >> "%LOGFILE%"

:: Run the PowerShell script with appropriate PowerShell version
if %PS64_AVAILABLE% equ 1 (
    "%PS64_PATH%" -NoProfile -ExecutionPolicy Bypass ^
        -Command "& '%PS_SCRIPT%' *>> '%LOGFILE%' 2>&1"
) else (
    powershell.exe -NoProfile -ExecutionPolicy Bypass ^
        -Command "& '%PS_SCRIPT%' *>> '%LOGFILE%' 2>&1"
)

set PS_EXITCODE=%errorlevel%

echo.                                                            >> "%LOGFILE%"
echo [%DATE% %TIME%] PowerShell script finished.                  >> "%LOGFILE%"
echo [%DATE% %TIME%] Exit code: !PS_EXITCODE!                     >> "%LOGFILE%"

if !PS_EXITCODE! equ 0 (
    echo [%DATE% %TIME%] Result: SUCCESS                              >> "%LOGFILE%"
    echo.
    echo Firstboot initialization completed successfully.
    echo Log saved to:
    echo   %LOGFILE%
) else (
    echo [%DATE% %TIME%] Result: FAILED (exit code !PS_EXITCODE!)     >> "%LOGFILE%"
    echo.
    echo Firstboot initialization FAILED (exit code !PS_EXITCODE!).
    echo Check the log for details:
    echo   %LOGFILE%
    echo.
    echo Attempting to reschedule using PowerShell scheduling mechanism...
    
    :: Reschedule using the same mechanism as in Firstboot-Scheduler.ps1
    if %PS64_AVAILABLE% equ 1 (
        "%PS64_PATH%" -NoProfile -ExecutionPolicy Bypass -Command ^
            "$Action = New-ScheduledTaskAction -Execute '%PS64_PATH%' -Argument '-NoProfile -ExecutionPolicy Bypass -File \"%PS_SCRIPT%\"'; " ^
            "$Trigger = New-ScheduledTaskTrigger -AtStartup; " ^
            "$Principal = New-ScheduledTaskPrincipal -UserId 'NT AUTHORITY\SYSTEM' -LogonType ServiceAccount -RunLevel Highest; " ^
            "$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 30); " ^
            "if (Get-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -ErrorAction SilentlyContinue) { Unregister-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -Confirm:$false; Start-Sleep -Seconds 2 }; " ^
            "Register-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -Description 'Firstboot Scheduler' -Force; " ^
            "exit $LASTEXITCODE" >nul 2>&1
    ) else (
        powershell -NoProfile -ExecutionPolicy Bypass -Command ^
            "$Action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -ExecutionPolicy Bypass -File \"%PS_SCRIPT%\"'; " ^
            "$Trigger = New-ScheduledTaskTrigger -AtStartup; " ^
            "$Principal = New-ScheduledTaskPrincipal -UserId 'NT AUTHORITY\SYSTEM' -LogonType ServiceAccount -RunLevel Highest; " ^
            "$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 30); " ^
            "if (Get-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -ErrorAction SilentlyContinue) { Unregister-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -Confirm:$false; Start-Sleep -Seconds 2 }; " ^
            "Register-ScheduledTask -TaskName 'FirstbootScheduler-PostReboot' -Action $Action -Trigger $Trigger -Principal $Principal -Settings $Settings -Description 'Firstboot Scheduler' -Force; " ^
            "exit $LASTEXITCODE" >nul 2>&1
    )
    
    if %errorlevel% equ 0 (
        echo [%DATE% %TIME%] Successfully rescheduled task for next boot >> "%LOGFILE%"
        echo.
        echo Script has been rescheduled to run at next boot.
        echo Please restart the computer.
    ) else (
        echo [%DATE% %TIME%] Failed to reschedule task                   >> "%LOGFILE%"
        echo.
        echo Failed to reschedule the script.
    )
)

echo.
echo [%DATE% %TIME%] Finished. Press any key to exit...           >> "%LOGFILE%"
pause >nul
exit /b !PS_EXITCODE!