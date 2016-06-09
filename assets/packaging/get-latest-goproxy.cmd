@echo off

setlocal enableextensions
setlocal enabledelayedexpansion

cd /d "%~dp0"

echo. >"%TEMP%\gdownload.vbs"
echo Set Http = CreateObject("WinHttp.WinHttpRequest.5.1") >>"%TEMP%\gdownload.vbs"
echo Set Stream = CreateObject("Adodb.Stream") >>"%TEMP%\gdownload.vbs"
echo Http.SetTimeouts 30*1000, 30*1000, 30*1000, 120*1000  >>"%TEMP%\gdownload.vbs"
echo Http.Open "GET", WScript.Arguments.Item(0), False >>"%TEMP%\gdownload.vbs"
echo Http.Send >>"%TEMP%\gdownload.vbs"
echo Stream.Type = 1 >>"%TEMP%\gdownload.vbs"
echo Stream.Open >>"%TEMP%\gdownload.vbs"
echo Stream.Write Http.ResponseBody >>"%TEMP%\gdownload.vbs"
echo Stream.SaveToFile WScript.Arguments.Item(1), 2 >>"%TEMP%\gdownload.vbs"


set has_user_json=0
if exist "httpproxy.json" (
    for %%I in (*.user.json) do (
        set has_user_json=1
    )
    if "!has_user_json!" == "0" (
        echo Please backup your config as .user.json
        goto quit
    )
)

set filename=
(
    echo 1. Checking GoProxy Version
    cscript /nologo "%TEMP%\gdownload.vbs" https://github.com/phuslu/goproxy/releases/tag/goproxy ~goproxy_tag.txt
) && (
    for /F "tokens=3 delims=<>" %%I in ('findstr "<strong>goproxy_windows_amd64-r" ~goproxy_tag.txt') do (
        set filename=%%I
    )
)
del /f ~goproxy_tag.txt
if "%filename%" == "" (
    echo Cannot detect goproxy_windows_amd64 version
    goto quit
)

(
    echo 2. Downloading %filename%
    cscript /nologo "%TEMP%\gdownload.vbs" https://github.com/phuslu/goproxy/releases/download/goproxy/%filename% "~%filename%"
    if not exist "~%filename%" (
        echo Cannot download %filename%
        goto quit
    )
) && (
    echo 3. Downloading 7za.exe for extracting
    cscript /nologo "%TEMP%\gdownload.vbs" https://github.com/phuslu/goproxy/releases/download/assets/7za.exe ~7za.exe
    if not exist "~7za.exe" (
        echo Cannot download 7za.exe
        goto quit
    )
) && (
    echo 4. Checking Goproxy program
:checkgoproxyprogram
    tasklist /NH /FI "IMAGENAME eq goproxy.exe" | findstr "goproxy.exe" >NUL && (
        echo %TIME% Please quit GoProxy program.
        ping -n 2 127.0.0.1 >NUL
        goto checkgoproxyprogram
    )
    echo 5. Extract Goproxy files
    ~7za.exe x -y ~%filename%
    del /f ~7za.exe ~%filename%
) && (
    echo 6. Done
)

:quit
    del /f ~* 2>NUL
    pause >NUL
