#!/bin/bash

set -e

cd $(python -c "import os; print(os.path.dirname(os.path.realpath('$0')))")

if [ -f "httpproxy.json" ]; then
    if ! ls *.user.json ; then
        echo "Please backup your config as .user.json"
        exit -1
    fi
fi

LOCALVERSION=$(./goproxy -version 2>/dev/null || :)
echo "0. Local Goproxy version $LOCALVERSION"

echo "1. Checking GoProxy Version"
REMOTEVERSION=$(curl https://github.com/phuslu/goproxy/releases/tag/goproxy | grep -oP '<strong>\Kgoproxy_linux_amd64-\Kr\d+')
if [ -z "${REMOTEVERSION}" ]; then
    echo "Cannot detect goproxy_linux_amd64 version"
    exit -1
fi

if [ x"$LOCALVERSION" == x"$REMOTEVERSION" ]; then
	echo "Your GoProxy already update to latest"
	exit -1
fi

echo "2. Downloading ${FILENAME}"
FILENAME=goproxy_linux_amd64-$REMOTEVERSION.tar.xz
curl -kL https://github.com/phuslu/goproxy/releases/download/goproxy/${FILENAME} | xz -d | tar xvp --strip-components=1

echo "3. Done"
