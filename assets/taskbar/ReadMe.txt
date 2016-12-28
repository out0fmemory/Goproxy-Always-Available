i686-w64-mingw32-windres taskbar.rc -O coff -o taskbar.res
i686-w64-mingw32-gcc -Wall -Os -O3 -m32 -s -std=c99 -Wl,--subsystem,windows -c taskbar.c
i686-w64-mingw32-gcc -static -Os -s -o goproxy-gui.exe taskbar.o taskbar.res -lstdc++ -lwininet