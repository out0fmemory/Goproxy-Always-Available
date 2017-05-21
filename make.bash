#!/bin/bash

set -ex

REVSION=$(git rev-list --count HEAD)
LDFLAGS="-s -w -X main.version=r${REVSION}"

GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}
GOARM=${GOARM:-5}
CGO_ENABLED=${CGO_ENABLED:-$(go env CGO_ENABLED)}

REPO=$(git rev-parse --show-toplevel)
PACKAGE=goproxy-vps
if [ "${CGO_ENABLED}" = "0" ]; then
    BUILDROOT=${REPO}/build/${GOOS}_${GOARCH}
else
    BUILDROOT=${REPO}/build/${GOOS}_${GOARCH}_cgo
fi
STAGEDIR=${BUILDROOT}/stage
OBJECTDIR=${BUILDROOT}/obj
DISTDIR=${BUILDROOT}/dist

if [ "${GOOS}" == "windows" ]; then
    GOPROXY_EXE="${PACKAGE}.exe"
    GOPROXY_STAGEDIR="${STAGEDIR}"
    GOPROXY_DISTCMD="7za a -y -mx=9 -m0=lzma -mfb=128 -md=64m -ms=on"
    GOPROXY_DISTEXT=".7z"
elif [ "${GOOS}" == "darwin" ]; then
    GOPROXY_EXE="${PACKAGE}"
    GOPROXY_STAGEDIR="${STAGEDIR}"
    GOPROXY_DISTCMD="env BZIP=-9 tar cvjpf"
    GOPROXY_DISTEXT=".tar.bz2"
elif [ "${GOARCH:0:3}" == "arm" ]; then
    GOPROXY_EXE="${PACKAGE}"
    GOPROXY_STAGEDIR="${STAGEDIR}"
    GOPROXY_DISTCMD="env BZIP=-9 tar cvjpf"
    GOPROXY_DISTEXT=".tar.bz2"
elif [ "${GOARCH:0:4}" == "mips" ]; then
    GOPROXY_EXE="${PACKAGE}"
    GOPROXY_STAGEDIR="${STAGEDIR}"
    GOPROXY_DISTCMD="env GZIP=-9 tar cvzpf"
    GOPROXY_DISTEXT=".tar.gz"
else
    GOPROXY_EXE="${PACKAGE}"
    GOPROXY_STAGEDIR="${STAGEDIR}/${PACKAGE}"
    GOPROXY_DISTCMD="env XZ_OPT=-9 tar cvJpf"
    GOPROXY_DISTEXT=".tar.xz"
fi

GOPROXY_DIST=${DISTDIR}/${PACKAGE}_${GOOS}_${GOARCH}-r${REVSION}${GOPROXY_DISTEXT}
if [ "${CGO_ENABLED}" = "1" ]; then
    GOPROXY_DIST=${DISTDIR}/${PACKAGE}_${GOOS}_${GOARCH}_cgo-r${REVSION}${GOPROXY_DISTEXT}
fi

GOPROXY_GUI_EXE=${REPO}/assets/taskbar/${GOARCH}/goproxy-gui.exe
if [ ! -f "${GOPROXY_GUI_EXE}" ]; then
    GOPROXY_GUI_EXE=${REPO}/assets/packaging/goproxy-gui.exe
fi

OBJECTS=${OBJECTDIR}/${GOPROXY_EXE}

SOURCES="${REPO}/README.md \
         ${REPO}/get-latest-goproxy-vps.sh \
         ${REPO}/goproxy-vps.sh \
         ${REPO}/goproxy-vps.toml \
         ${REPO}/pwauth"

build () {
    mkdir -p ${OBJECTDIR}
    env GOOS=${GOOS} \
        GOARCH=${GOARCH} \
        GOARM=${GOARM} \
        CGO_ENABLED=${CGO_ENABLED} \
    go build -v -ldflags="${LDFLAGS}" -o ${OBJECTDIR}/${GOPROXY_EXE} .
}

dist () {
    mkdir -p ${DISTDIR} ${STAGEDIR} ${GOPROXY_STAGEDIR}
    cp ${OBJECTS} ${SOURCES} ${GOPROXY_STAGEDIR}

    pushd ${STAGEDIR}
    ${GOPROXY_DISTCMD} ${GOPROXY_DIST} *
    popd
}

clean () {
    rm -rf ${BUILDROOT}
}

case $1 in
    build)
        build
        ;;
    dist)
        dist
        ;;
    clean)
        clean
        ;;
    *)
        build
        dist
        ;;
esac
