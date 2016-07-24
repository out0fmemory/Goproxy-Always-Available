#!/bin/bash

set -e

cd $(python -c "import os; print(os.path.dirname(os.path.realpath('$0')))")

if [ -f "httpproxy.json" ]; then
    if ! ls *.user.json ; then
        echo "Please backup your config as .user.json"
        exit -1
    fi
fi

FILENAME_PREFIX=
FILENAME_SUFFIX=
case $(uname -s)/$(uname -m) in
	Linux/x86_64 )
		FILENAME_PREFIX=goproxy_linux_amd64-
		FILENAME_SUFFIX=.tar.xz
		FILENAME_DISTCMD="xz -d"
		;;
	Linux/i686|Linux/i386 )
		FILENAME_PREFIX=goproxy_linux_386-
		FILENAME_SUFFIX=.tar.xz
		FILENAME_DISTCMD="xz -d"
		;;
	Linux/armv7l|Linux/armv8 )
		FILENAME_PREFIX=goproxy_linux_arm64-
		FILENAME_SUFFIX=.tar.xz
		FILENAME_DISTCMD="xz -d"
		;;
	Linux/arm* )
		FILENAME_PREFIX=goproxy_linux_arm-
		FILENAME_SUFFIX=.tar.xz
		FILENAME_DISTCMD="xz -d"
		;;
	Linux/mips64el )
		FILENAME_PREFIX=goproxy_linux_mips64le-
		FILENAME_SUFFIX=.tar.xz
		FILENAME_DISTCMD="xz -d"
		;;
	Linux/mips64 )
		FILENAME_PREFIX=goproxy_linux_mips64-
		FILENAME_SUFFIX=.tar.xz
		FILENAME_DISTCMD="xz -d"
		;;
	FreeBSD/x86_64 )
		FILENAME_PREFIX=goproxy_freebsd_amd64-
		FILENAME_SUFFIX=.tar.bz2
		FILENAME_DISTCMD="xz -d"
		;;
	FreeBSD/i686|FreeBSD/i386 )
		FILENAME_PREFIX=goproxy_freebsd_386-
		FILENAME_SUFFIX=.tar.bz2
		FILENAME_DISTCMD="xz -d"
		;;
	Darwin/x86_64 )
		FILENAME_PREFIX=goproxy_macos_amd64-
		FILENAME_SUFFIX=.tar.bz2
		FILENAME_DISTCMD="bzip2 -d"
		;;
	Darwin/i686|Darwin/i386 )
		FILENAME_PREFIX=goproxy_macos_386-
		FILENAME_SUFFIX=.tar.bz2
		FILENAME_DISTCMD="bzip2 -d"
		;;
	* )
		echo "Unsupported platform: $(uname -a)"
		exit -1
		;;
esac

LOCALVERSION=$(./goproxy -version 2>/dev/null || :)
echo "0. Local Goproxy version $LOCALVERSION"

echo "1. Checking GoProxy Version"
REMOTEVERSION=$(curl https://github.com/phuslu/goproxy/releases/tag/goproxy | grep -oP "<strong>\K${FILENAME_PREFIX}\Kr\d+")
if [ -z "${REMOTEVERSION}" ]; then
    echo "Cannot detect $FILENAME_PREFIX version"
    exit -1
fi

if [ x"$LOCALVERSION" == x"$REMOTEVERSION" ]; then
	echo "Your GoProxy already update to latest"
	exit -1
fi

echo "2. Downloading ${FILENAME}"
FILENAME=${FILENAME_PREFIX}${REMOTEVERSION}${FILENAME_SUFFIX}
curl -kL https://github.com/phuslu/goproxy/releases/download/goproxy/${FILENAME} | ${FILENAME_DISTCMD} | tar xvp --strip-components=1

echo "3. Done"
