x86_64-w64-mingw32-windres taskbar.rc -O coff -o taskbar.res
x86_64-w64-mingw32-g++ -Wall -Os -s -Wl,--subsystem,windows -c taskbar.c
x86_64-w64-mingw32-g++ -static -Os -s -o goproxy-gui.exe taskbar.o taskbar.res -lwininet