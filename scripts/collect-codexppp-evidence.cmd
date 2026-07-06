@echo off
setlocal
set SCRIPT=%~dp0collect-codexppp-evidence.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%"
echo.
pause
