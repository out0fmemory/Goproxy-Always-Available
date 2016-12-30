#!/bin/bash

export LATEST=${LATEST:-false}

set -e

WHICH=$(which which || echo 'type -p')

for CMD in which curl sed expr tar;
do
		if ! ${WHICH} ${CMD} >/dev/null; then
				echo "tool ${CMD} is not installed, abort."
				exit 1
		fi
done

linkpath=$(ls -l "$0" | sed "s/.*->\s*//")
cd "$(dirname "$0")" && test -f "$linkpath" && cd "$(dirname "$linkpath")" || true

if [ -f "httpproxy.json" ]; then
	if ! ls *.user.json ; then
		echo "Please backup your config as .user.json"
		exit 1
	fi
fi

if [ -d cache ]; then
	find cache -name "*.crt" -mtime +90 -delete
fi

FILENAME_PREFIX=
case $(uname -s)/$(uname -m) in
	Linux/x86_64 )
		FILENAME_PREFIX=goproxy_linux_amd64
		;;
	Linux/i686|Linux/i386 )
		FILENAME_PREFIX=goproxy_linux_386
		;;
	Linux/aarch64|Linux/arm64 )
		FILENAME_PREFIX=goproxy_linux_arm64
		;;
	Linux/arm* )
		FILENAME_PREFIX=goproxy_linux_arm
		if grep -q ld-linux-armhf.so ./goproxy; then
			FILENAME_PREFIX=goproxy_linux_arm_cgo
		fi
		;;
	Linux/mips64el )
		FILENAME_PREFIX=goproxy_linux_mips64le
		;;
	Linux/mips64 )
		FILENAME_PREFIX=goproxy_linux_mips64
		;;
	Linux/mipsel )
		FILENAME_PREFIX=goproxy_linux_mipsle
		;;
	Linux/mips )
		FILENAME_PREFIX=goproxy_linux_mips
		;;
	FreeBSD/x86_64 )
		FILENAME_PREFIX=goproxy_freebsd_amd64
		;;
	FreeBSD/i686|FreeBSD/i386 )
		FILENAME_PREFIX=goproxy_freebsd_386
		;;
	Darwin/x86_64 )
		FILENAME_PREFIX=goproxy_macos_amd64
		;;
	Darwin/i686|Darwin/i386 )
		FILENAME_PREFIX=goproxy_macos_386
		;;
	* )
		echo "Unsupported platform: $(uname -a)"
		exit 1
		;;
esac

LOCALVERSION=$(./goproxy -version 2>/dev/null || :)
echo "0. Local Goproxy version ${LOCALVERSION}"

if test "${http_proxy}" = ""; then
	if netstat -an | grep -i tcp | grep LISTEN | grep '[:\.]8087'; then
		echo "Set http_proxy=http://127.0.0.1:8087"
		export http_proxy=http://127.0.0.1:8087
		export https_proxy=http://127.0.0.1:8087
	fi
fi

for USER_JSON_FILE in *.user.json; do
	USER_JSON_LINE=$(head -1 ${USER_JSON_FILE} | tr -d '\r')
	if echo "${USER_JSON_LINE}" | grep -q AUTO_UPDATE_URL; then
		USER_JSON_URL=${USER_JSON_LINE#* }
		echo "Update ${USER_JSON_FILE} with ${USER_JSON_URL}"
		curl -k "${USER_JSON_URL}" >${USER_JSON_FILE}.tmp
		mv ${USER_JSON_FILE}.tmp ${USER_JSON_FILE}
	fi
done

echo "1. Checking GoProxy Version"
if test "${LATEST}" = "false"; then
	FILENAME=$(curl -k https://github.com/phuslu/goproxy-ci/commits/master | grep -oE "${FILENAME_PREFIX}-r[0-9]+.[0-9a-z\.]+" | head -1)
else
	FILENAME=$(curl -kL https://github.com/phuslu/goproxy-ci/releases/latest | grep -oE "${FILENAME_PREFIX}-r[0-9]+.[0-9a-z\.]+" | head -1)
fi
REMOTEVERSION=$(echo ${FILENAME} | awk -F'.' '{print $1}' | awk -F'-' '{print $2}')
if test -z "${REMOTEVERSION}"; then
	echo "Cannot detect ${FILENAME_PREFIX} version"
	exit 1
fi

if expr "${LOCALVERSION#r*}" ">=" "${REMOTEVERSION#r*}" >/dev/null; then
	echo "Your GoProxy already update to latest"
	exit 1
fi

echo "2. Downloading ${FILENAME}"
curl -kL https://github.com/phuslu/goproxy-ci/releases/download/${REMOTEVERSION}/${FILENAME} >${FILENAME}.tmp
mv -f ${FILENAME}.tmp ${FILENAME}

echo "3. Extracting ${FILENAME}"
rm -rf ${FILENAME%.*}
case ${FILENAME##*.} in
	xz )
		xz -d ${FILENAME}
		;;
	bz2 )
		bzip2 -d ${FILENAME}
		;;
	gz )
		gzip -d ${FILENAME}
		;;
	* )
		echo "Unsupported archive format: ${FILENAME}"
		exit 1
esac

tar -xvpf ${FILENAME%.*} --strip-components $(tar -tf ${FILENAME%.*} | head -1 | grep -c '/')
rm -f ${FILENAME%.*}

echo "4. Done"
