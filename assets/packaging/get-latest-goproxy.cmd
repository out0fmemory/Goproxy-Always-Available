@echo off

setlocal enableextensions
setlocal enabledelayedexpansion

cd /d "%~dp0"

set has_user_json=0
if exist "httpproxy.json" (
    for %%I in (*.user.json) do (
        set has_user_json=1
    )
    if "%has_user_json%" == "0" (
        echo Please backup your config as .user.json
        pause >NUL
        exit /b -1
    )
)

set filename=
(
    echo 1. Checking GoProxy Version
    powershell -Command "Invoke-WebRequest https://github.com/phuslu/goproxy/releases/tag/goproxy -OutFile ~goproxy_tag.txt"
) && (
    for /F "tokens=3 delims=<>" %%I in ('findstr "<strong>goproxy_windows_amd64-r" ~goproxy_tag.txt') do (
        set filename=%%I
    )
)
del /f ~goproxy_tag.txt
if "%filename%" == "" (
    echo Cannot detect goproxy_windows_amd64 version
    pause >NUL
    exit /b -1
)

(
    echo 2. Downloading %filename%
    powershell -Command "Invoke-WebRequest https://github.com/phuslu/goproxy/releases/download/goproxy/%filename% -OutFile ~%filename%"
    echo 3. Downloading 7za.exe for extracting
    powershell -Command "Invoke-WebRequest https://github.com/phuslu/goproxy/releases/download/assets/7za.exe -OutFile ~7za.exe"
) && (
    echo 4. Checking Goproxy program
    tasklist /NH /FI "IMAGENAME eq goproxy.exe" | findstr "goproxy.exe" && (
        echo Please quit GoProxy program.
        pause >NUL
    )
    echo 5. Extract Goproxy files
    ~7za.exe x -y ~%filename%
    del /f ~7za.exe ~%filename%
) && (
    echo 6. Done
) || (
    del /f ~*
)
pause >NUL
