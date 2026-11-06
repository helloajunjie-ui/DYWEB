@echo off
setlocal
cd /d "%~dp0"
if not exist "dyweb.exe" (
  echo [ERROR] dyweb.exe not found.
  pause
  exit /b 1
)
start "DYweb" /min "dyweb.exe" -root "%~dp0"
timeout /t 2 /nobreak >nul
start "" "http://127.0.0.1:9101/"
timeout /t 5 >nul
endlocal