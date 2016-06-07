#!/bin/bash

set -e

if [ -f "httpproxy.json" ]; then
    if ! ls *.user.json ; then
        echo "Please backup your config as .user.json"
        exit -1
    fi
fi

echo "1. Checking GoProxy Version"
FILENAME=$(curl https://github.com/phuslu/goproxy/releases/tag/goproxy | grep -oP '<strong>\Kgoproxy_linux_amd64-r\d+\.tar.xz')
if [ -z "${FILENAME}" ]; then
    echo "Cannot detect goproxy_linux_amd64 version"
    exit -1
fi

echo "2. Downloading ${FILENAME}"
curl -kL https://github.com/phuslu/goproxy/releases/download/goproxy/${FILENAME} | xz -d | tar xvp

echo "3. Done"
