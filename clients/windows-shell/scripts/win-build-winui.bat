@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0win-build-winui.ps1" %*
exit /b %ERRORLEVEL%
