GOARCH ?= amd64

ifeq ($(GOARCH), amd64)
	SUFFIX=
	WINDRES=x86_64-w64-mingw32-windres
	CXX=x86_64-w64-mingw32-g++
else
	SUFFIX=_x86
	WINDRES=i686-w64-mingw32-windres
	CXX=i686-w64-mingw32-g++
endif

ifeq (, $(shell command -v $(WINDRES) 2>/dev/null))
	WINDRES=windres
endif

.PHONY: build
build:
	$(WINDRES) taskbar.rc -O coff -o taskbar$(SUFFIX).res
	$(CXX) -Wall -Os -s -Wl,--subsystem,windows -o taskbar$(SUFFIX).o -c taskbar.c
	$(CXX) -static -Os -s -o goproxy-gui$(SUFFIX).exe taskbar$(SUFFIX).o taskbar$(SUFFIX).res -lwininet

.PHONY: clean
clean:
	$(RM) taskbar*.res
	$(RM) taskbar*.o
	$(RM) goproxy-gui*.exe
