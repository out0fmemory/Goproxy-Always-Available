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
		;;
	Linux/i686|Linux/i386 )
		FILENAME_PREFIX=goproxy_linux_386-
		FILENAME_SUFFIX=.tar.xz
		;;
	Linux/armv7l|Linux/armv8 )
		FILENAME_PREFIX=goproxy_linux_arm64-
		FILENAME_SUFFIX=.tar.xz
		;;
	Linux/arm* )
		FILENAME_PREFIX=goproxy_linux_arm-
		FILENAME_SUFFIX=.tar.xz
		;;
	Linux/mips64el )
		FILENAME_PREFIX=goproxy_linux_mips64le-
		FILENAME_SUFFIX=.tar.xz
		;;
	Linux/mips64 )
		FILENAME_PREFIX=goproxy_linux_mips64-
		FILENAME_SUFFIX=.tar.xz
		;;
	FreeBSD/x86_64 )
		FILENAME_PREFIX=goproxy_freebsd_amd64-
		FILENAME_SUFFIX=.tar.bz2
		;;
	FreeBSD/i686|FreeBSD/i386 )
		FILENAME_PREFIX=goproxy_freebsd_386-
		FILENAME_SUFFIX=.tar.bz2
		;;
	Darwin/x86_64 )
		FILENAME_PREFIX=goproxy_macos_amd64-
		FILENAME_SUFFIX=.tar.bz2
		;;
	Darwin/i686|Darwin/i386 )
		FILENAME_PREFIX=goproxy_macos_386-
		FILENAME_SUFFIX=.tar.bz2
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

FILENAME=${FILENAME_PREFIX}${REMOTEVERSION}${FILENAME_SUFFIX}

echo "2. Downloading ${FILENAME}"
curl -kL https://github.com/phuslu/goproxy/releases/download/goproxy/${FILENAME} | ([[ "${FILENAME_SUFFIX}" == *.xz ]] && xz -d || bzip2 -d)  | tar xvp --strip-components=1

echo "3. Done"
