i686-w64-mingw32-windres taskbar.rc -O coff -o taskbar.res
i686-w64-mingw32-g++ -Os -O3 -m32 -s -fno-exceptions -fno-rtti -fno-ident -flto -nostdlib -c -Wall taskbar.cpp
i686-w64-mingw32-g++ -static -o goproxy-gui.exe taskbar.o taskbar.res -lkernel32 -luser32 -lrasapi32 -lshell32 -lpsapi -ladvapi32 -lwininet
