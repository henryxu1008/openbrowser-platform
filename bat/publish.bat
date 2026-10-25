@echo off
setlocal EnableExtensions

set "SCRIPT_DIR=%~dp0"
where pwsh >nul 2>nul
if errorlevel 1 (
    echo [ERROR] Publishing requires PowerShell 7 ^(pwsh^). End users do not need it.
    endlocal & exit /b 1
)
if not exist "%SCRIPT_DIR%publish.ps1" (
    echo [ERROR] Missing bat\publish.ps1
    if /I not "%NO_PAUSE%"=="1" if /I not "%CI%"=="1" pause
    endlocal & exit /b 1
)

if /I "%~1"=="zip" (
    shift /1
    pwsh -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%publish.ps1" -Target WINDOWS -WindowsFormat PORTABLE %*
    goto :after_publish
)
if /I "%~1"=="portable" (
    shift /1
    pwsh -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%publish.ps1" -Target WINDOWS -WindowsFormat PORTABLE %*
    goto :after_publish
)
if /I "%~1"=="both" (
    shift /1
    pwsh -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%publish.ps1" -Target WINDOWS -WindowsFormat BOTH %*
    goto :after_publish
)

pwsh -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%publish.ps1" %*

:after_publish
set "EXIT_CODE=%ERRORLEVEL%"

echo.
if "%EXIT_CODE%"=="0" (
    echo Publish finished successfully.
) else (
    echo Publish failed with exit code %EXIT_CODE%.
)

if /I not "%NO_PAUSE%"=="1" if /I not "%CI%"=="1" pause

endlocal & exit /b %EXIT_CODE%
