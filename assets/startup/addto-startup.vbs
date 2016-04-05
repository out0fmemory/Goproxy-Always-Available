' 
' Run as Administrator and UAC
' 解决脚本在windows7以上系统运行问题
' 在XP下运行问题：会弹出一个"运行身份"对话框，
'   "保护我的计算机和数据不受未授权程序的活动影响"默认是勾选的，须取消。然后，确定运行。正常。
' (注：2016年了，现在很了人用XP了吧。)
Option Explicit
Dim wsh, fso, ObjShell, BtnCode, ScriptDir, FilePath, link

Set wsh = WScript.CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")

' 出处
' https://groups.google.com/forum/#!topic/microsoft.public.scripting.vbscript/Fb-YibxZ2x8
' https://stackoverflow.com/questions/13296281/permission-elevation-from-vbscript
'
If WScript.Arguments.length = 0 Then
    Set ObjShell = CreateObject("Shell.Application")
    ObjShell.ShellExecute "wscript.exe", """" & _
                            WScript.ScriptFullName & """" &_
                            " RunAsAdministrator", , "runas", 1
    WScript.Quit
End If


Function CreateShortcut(FilePath)
    Set wsh = WScript.CreateObject("WScript.Shell")
    Set link = wsh.CreateShortcut(wsh.SpecialFolders("Startup") + "\goproxy.lnk")
    link.TargetPath = FilePath
    link.Arguments = ""
    link.WindowStyle = 7
    link.Description = "GoProxy"
    link.WorkingDirectory = wsh.CurrentDirectory
    link.Save()
End Function

BtnCode = wsh.Popup("是否将 goproxy.exe 加入到启动项？(本对话框 6 秒后消失)", 6, "GoProxy 对话框", 1+32)
If BtnCode = 1 Then
    ScriptDir = CreateObject("Scripting.FileSystemObject").GetParentFolderName(WScript.ScriptFullName)
    FilePath = ScriptDir + "\goproxy-gui.exe"
    If Not fso.FileExists(FilePath) Then
        wsh.Popup "当前目录下不存在 goproxy-gui.exe ", 5, "GoProxy 对话框", 16
        WScript.Quit
    End If
    CreateShortcut(FilePath)
    wsh.Popup "成功加入 GoProxy 到启动项", 5, "GoProxy 对话框", 64
End If
