#!/bin/bash

set -ex

export GOARCH=${GOARCH:-amd64}

if [ "${GOARCH}" = "amd64" ]; then
    BUILDROOT=amd64
    WINDRES=x86_64-w64-mingw32-windres
    CXX=x86_64-w64-mingw32-g++
else
    BUILDROOT=386
    WINDRES=i686-w64-mingw32-windres
    CXX=i686-w64-mingw32-g++
fi

if ! ${WINDRES} --version >/dev/null; then
    WINDRES=windres
fi

build () {
    mkdir -p ${BUILDROOT}
    ${WINDRES} taskbar.rc -O coff -o ${BUILDROOT}/taskbar.res
    ${CXX} -Wall -Os -s -Wl,--subsystem,windows -o ${BUILDROOT}/taskbar.o -c taskbar.c
    ${CXX} -static -Os -s -o ${BUILDROOT}/goproxy-gui.exe ${BUILDROOT}/taskbar.o ${BUILDROOT}/taskbar.res -lwininet
}

clean () {
    rm -rf ${BUILDROOT}
}

case $1 in
    build)
        build
        ;;
    clean)
        clean
        ;;
    *)
        build
        ;;
esac
