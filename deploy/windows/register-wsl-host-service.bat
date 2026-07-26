@echo off
rem Self-elevating launcher for register-wsl-host-service.ps1.
rem All arguments are forwarded to the PowerShell script once elevated, so
rem interactive prompts (Get-Credential) run inside a real elevated
rem PowerShell process instead of failing under a non-elevated cmd.exe.
rem Must be run from a terminal with arguments -- Explorer launches (double
rem click / "Run as administrator") cannot pass command-line arguments.
rem
rem Elevation uses PowerShell's own Start-Process -Verb RunAs -Wait, which
rem blocks until the elevated process (including any Get-Credential prompt)
rem actually exits. Do not replace this with a non-blocking elevation
rem helper -- one that does not wait reports success as soon as the
rem elevated process is *requested*, not once registration has actually
rem completed or the UAC prompt has been answered.
rem
rem The forwarded call is written to a temp .ps1 file rather than passed as
rem trailing arguments to another "powershell -Command ... %*" hop: quoted
rem arguments containing spaces (e.g. "C:\Program Files\...") do not survive
rem a second round of command-line re-parsing by powershell.exe. Writing
rem %* (already correctly quoted by cmd.exe) straight into a script file and
rem executing that file sidesteps the re-parsing entirely.
rem
rem Start-Process -Verb RunAs opens the elevated process in its own window
rem and cannot be combined with -RedirectStandardOutput/-RedirectStandardError
rem (a .NET restriction on the "runas" verb), so -Wait alone gives no visibility
rem into what happened inside it. The generated script wraps the real call in
rem Start-Transcript so failures (including a cancelled/timed-out UAC prompt)
rem can be diagnosed from %TEMP%\register-wsl-host-service-elevate.log.
setlocal

openfiles > nul 2>nul
if errorlevel 1 goto :elevate
goto :run

:elevate
set "elevscript=%temp%\register-wsl-host-service-elevate.ps1"
set "elevlog=%temp%\register-wsl-host-service-elevate.log"
> "%elevscript%" echo Start-Transcript -Path '%elevlog%' -Force ^> $null
>> "%elevscript%" echo ^& '%~dp0register-wsl-host-service.ps1' %*
>> "%elevscript%" echo Stop-Transcript ^> $null
powershell -NoProfile -Command "Start-Process powershell -Verb RunAs -Wait -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','%elevscript%'"
set "exitcode=%errorlevel%"
del "%elevscript%"
echo Elevated run log: %elevlog%
exit /b %exitcode%

:run
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0register-wsl-host-service.ps1" %*
set "exitcode=%errorlevel%"
pause
exit /b %exitcode%
