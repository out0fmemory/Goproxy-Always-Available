@echo off

setlocal enableextensions
setlocal enabledelayedexpansion

cd /d "%~dp0"

echo. >~gdownload.vbs
echo Set Http = CreateObject("WinHttp.WinHttpRequest.5.1") >>~gdownload.vbs
echo Set Stream = CreateObject("Adodb.Stream") >>~gdownload.vbs
echo Http.SetTimeouts 30*1000, 30*1000, 30*1000, 120*1000  >>~gdownload.vbs
netstat -an| findstr LISTENING | findstr ":8087" >NUL && (
    echo Http.SetProxy 2, "127.0.0.1:8087", "" >>~gdownload.vbs
)
echo Http.Open "GET", WScript.Arguments.Item(0), False >>~gdownload.vbs
echo Http.Send >>~gdownload.vbs
echo Stream.Type = 1 >>~gdownload.vbs
echo Stream.Open >>~gdownload.vbs
echo Stream.Write Http.ResponseBody >>~gdownload.vbs
echo Stream.SaveToFile WScript.Arguments.Item(1), 2 >>~gdownload.vbs


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

set filename_pattern=goproxy_windows_386
if exist "%systemdrive%\Program Files (x86)" (
    set filename_pattern=goproxy_windows_amd64
)


set localname=
if exist "goproxy.exe" (
    for /f "usebackq" %%I in (`goproxy.exe -version`) do (
        echo %%I | findstr /r "r[0-9][0-9][0-9][0-9]*" >NUL && (
            set localname=!filename_pattern!-%%I.7z
        )
    )
)
if not "%localname%" == "" (
    echo 0. Local Goproxy version %localname%
)

set filename=
(
    echo 1. Checking GoProxy Version
    cscript /nologo ~gdownload.vbs https://github.com/phuslu/goproxy/releases/tag/goproxy ~goproxy_tag.txt
) && (
    for /f "usebackq tokens=3 delims=<>" %%I in (`findstr "<strong>%filename_pattern%-r" ~goproxy_tag.txt`) do (
        set filename=%%I
    )
)
del /f ~goproxy_tag.txt
if "%filename%" == "" (
    echo Cannot detect %filename_pattern% version
    goto quit
)

if "%localname%" == "%filename%" (
    echo.
    echo Your Goproxy already update to latest.
    goto quit
)

(
    echo 2. Downloading %filename%
    cscript /nologo ~gdownload.vbs https://github.com/phuslu/goproxy/releases/download/goproxy/%filename% "~%filename%"
    if not exist "~%filename%" (
        echo Cannot download %filename%
        goto quit
    )
) && (
    echo 3. Downloading 7za.exe for extracting
    cscript /nologo ~gdownload.vbs https://github.com/phuslu/goproxy/releases/download/assets/7za.exe ~7za.exe
    if not exist "~7za.exe" (
        echo Cannot download 7za.exe
        goto quit
    )
) && (
    echo 4. Checking Goproxy program
:checkgoproxyprogram
    if exist "goproxy.exe" (
        tasklist /nh /fi "IMAGENAME eq goproxy.exe" | findstr "goproxy.exe" >NUL && (
            echo %TIME% Please quit GoProxy program.
            ping -n 2 127.0.0.1 >NUL
            goto checkgoproxyprogram
        )
    )
    echo 5. Extract Goproxy files
    ~7za.exe x -y ~%filename%
    echo 6. Done
)

:quit
    del /f ~* 2>NUL
    pause >NUL
