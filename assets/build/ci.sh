#!/bin/bash -xe

export GITHUB_USER=${GITHUB_USER:-phuslu}
export GITHUB_EMAIL=${GITHUB_EMAIL:-${GITHUB_USER}@noreply.github.com}
export GITHUB_REPO=${GITHUB_REPO:-goproxy}
export GITHUB_CI_REPO=${GITHUB_CI_REPO:-goproxy-ci}
export GITHUB_COMMIT_ID=${TRAVIS_COMMIT:-${COMMIT_ID:-master}}
export WORKING_DIR=$(pwd)/${GITHUB_REPO}.$(date "+%y%m%d").${RANDOM:-$$}
export GOROOT_BOOTSTRAP=${WORKING_DIR}/goroot_bootstrap
export GOROOT=${WORKING_DIR}/go
export GOPATH=${WORKING_DIR}/gopath
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
export GOBRANCH=${GOBRANCH:-master}
export CGO_ENABLED=${CGO_ENABLED:-0}

if [ ${#GITHUB_TOKEN} -eq 0 ]; then
	echo "WARNING: \$GITHUB_TOKEN is not set!"
fi

for CMD in curl awk git tar bzip2 xz 7za gcc make sha1sum timeout
do
	if ! type -p ${CMD}; then
		echo -e "\e[1;31mtool ${CMD} is not installed, abort.\e[0m"
		exit 1
	fi
done

mkdir -p ${WORKING_DIR}

function rename() {
	for FILENAME in ${2+"$@"}
	do
		local NEWNAME=$(echo ${FILENAME} | sed -r $1)
		if [ "${NEWNAME}" != "${FILENAME}" ]; then
			mv -f ${FILENAME} ${NEWNAME}
		fi
	done
}

function init_github() {
	pushd ${WORKING_DIR}

	git config --global user.name "${GITHUB_USER}"
	git config --global user.email "${GITHUB_EMAIL}"

	if ! grep -q 'machine github.com' ~/.netrc; then
		if [ ${#GITHUB_TOKEN} -gt 0 ]; then
			(set +x; echo "machine github.com login $GITHUB_USER password $GITHUB_TOKEN" >>~/.netrc)
		fi
	fi

	popd
}

function build_go() {
	pushd ${WORKING_DIR}

	curl -k https://storage.googleapis.com/golang/go1.7.1.linux-amd64.tar.gz | tar xz
	mv go goroot_bootstrap

	git clone --branch ${GOBRANCH} https://github.com/phuslu/go
	cd go/src
	git remote add -f upstream https://github.com/golang/go
	git rebase upstream/${GOBRANCH}
	bash ./make.bash
	grep -q 'machine github.com' ~/.netrc && git push -f origin ${GOBRANCH}

	(set +x; \
		echo '================================================================================' ;\
		cat /etc/issue ;\
		uname -a ;\
		echo ;\
		go version ;\
		go env ;\
		echo ;\
		env | grep -v GITHUB_TOKEN ;\
		echo '================================================================================' ;\
	)

	popd
}

function build_glog() {
	pushd ${WORKING_DIR}

	git clone https://github.com/phuslu/glog $GOPATH/src/github.com/phuslu/glog
	cd $GOPATH/src/github.com/phuslu/glog
	git remote add -f upstream https://github.com/golang/glog
	git rebase upstream/master
	go build -v
	grep -q 'machine github.com' ~/.netrc && git push -f origin master

	popd
}

function build_http2() {
	pushd ${WORKING_DIR}

	git clone https://github.com/phuslu/net $GOPATH/src/github.com/phuslu/net
	cd $GOPATH/src/github.com/phuslu/net/http2
	git remote add -f upstream https://github.com/golang/net
	git rebase upstream/master
	go build -v
	grep -q 'machine github.com' ~/.netrc && git push -f origin master

	popd
}

function build_repo() {
	pushd ${WORKING_DIR}

	git clone https://github.com/${GITHUB_USER}/${GITHUB_REPO} ${GITHUB_REPO}
	cd ${GITHUB_REPO}

	if [ ${TRAVIS_PULL_REQUEST:-false} == "false" ]; then
		git checkout -f ${GITHUB_COMMIT_ID}
	else
		git fetch origin pull/${TRAVIS_PULL_REQUEST}/head:pr
		git checkout -f pr
	fi

	export RELEASE=$(git rev-list --count HEAD)
	export RELEASE_DESCRIPTION=$(git log -1 --oneline --format="r${RELEASE}: [\`%h\`](https://github.com/${GITHUB_USER}/${GITHUB_REPO}/commit/%h) %s")
	if [ -n "${TRAVIS_BUILD_ID}" ]; then
		export RELEASE_DESCRIPTION=$(echo ${RELEASE_DESCRIPTION} | sed -E "s#^(r[0-9]+)#[\1](https://travis-ci.org/${GITHUB_USER}/${GITHUB_REPO}/builds/${TRAVIS_BUILD_ID})#g")
	fi

	if grep -lr $(printf '\r\n') * | grep '.go$' ; then
		echo -e "\e[1;31mPlease run dos2unix for go source files\e[0m"
		exit 1
	fi

	if [ "$(gofmt -l . | tee /dev/tty)" != "" ]; then
		echo -e "\e[1;31mPlease run 'gofmt -s -w .' for go source files\e[0m"
		exit 1
	fi

	awk 'match($1, /"((github\.com|golang\.org|gopkg\.in)\/.+)"/) {if (!seen[$1]++) {gsub("\"", "", $1); print $1}}' $(find . -name "*.go") | xargs -n1 -i go get -u -v {}

	go test -v ./httpproxy/helpers

	if curl -m 3 https://pki.google.com >/dev/null ; then
		GoogleG2PKP=$(curl -s https://pki.google.com/GIAG2.crt | openssl x509 -inform der -pubkey | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl base64)
		sed -i -r "s/\"GoogleG2PKP\": \".+\"/\"GoogleG2PKP\": \"$GoogleG2PKP\"/g" httpproxy/filters/gae/gae.json
		if git status -s | grep -q 'gae.json' ; then
			git diff
			git add httpproxy/filters/gae/gae.json
			git commit -m "update GoogleG2PKP to $GoogleG2PKP"
			grep -q 'machine github.com' ~/.netrc && git push -f origin master
		fi
	fi

	for OSARCH in darwin/386 \
				darwin/amd64 \
				freebsd/386 \
				freebsd/amd64 \
				linux/386 \
				linux/amd64 \
				linux/arm \
				linux/arm64 \
				linux/mips64 \
				linux/mips64le \
				windows/386 \
				windows/amd64
	do
		make GOOS=${OSARCH%/*} GOARCH=${OSARCH#*/}
		mkdir -p ${WORKING_DIR}/r${RELEASE}
		cp -r build/dist/* ${WORKING_DIR}/r${RELEASE}
		make clean
	done

	cd ${WORKING_DIR}/r${RELEASE}
	rename 's/_darwin_(amd64|386)/_macos_\1/' *
	rename 's/_darwin_(arm64|arm)/_ios_\1/' *
	ls -lht

	popd
}

function build_repo_ex() {
	pushd ${WORKING_DIR}/${GITHUB_REPO}

	git checkout -f server.vps

	awk 'match($1, /"((github\.com|golang\.org|gopkg\.in)\/.+)"/) {if (!seen[$1]++) {gsub("\"", "", $1); print $1}}' $(find . -name "*.go") | xargs -n1 -i go get -u -v {}

	make

	cp -r $(/bin/ls *.{gz,bz2,xz}) ${WORKING_DIR}/r${RELEASE}

	popd
}

function release_repo_ci() {
	pushd ${WORKING_DIR}

	if [ ${#GITHUB_TOKEN} -eq 0 ]; then
		echo -e "\e[1;31m\$GITHUB_TOKEN is not set, abort\e[0m"
		exit 1
	fi

	git clone --branch "master" https://github.com/${GITHUB_USER}/${GITHUB_CI_REPO} ${GITHUB_CI_REPO}
	cd ${GITHUB_CI_REPO}

	git commit --allow-empty -m "release"
	git tag -d r${RELEASE} || true
	git tag r${RELEASE}
	git push -f origin r${RELEASE}

	cd ${WORKING_DIR}
	local GITHUB_RELEASE_URL=https://github.com/aktau/github-release/releases/download/v0.6.2/linux-amd64-github-release.tar.bz2
	local GITHUB_RELEASE_BIN=$(pwd)/$(curl -L ${GITHUB_RELEASE_URL} | tar xjpv | head -1)

	cd ${WORKING_DIR}/r${RELEASE}/

	for i in $(seq 5)
	do
		if ! ${GITHUB_RELEASE_BIN} release --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} --name "${GITHUB_REPO} r${RELEASE}" --description "${RELEASE_DESCRIPTION}" ; then
			sleep 3
			${GITHUB_RELEASE_BIN} delete --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} >/dev/null 2>&1 || true
			sleep 3
			continue
		fi

		local allok="true"
		for FILE in *
		do
			if ! timeout -k60 60 ${GITHUB_RELEASE_BIN} upload --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} --name ${FILE} --file ${FILE} ; then
				allok="false"
				break
			fi
		done
		if [ "${allok}" == "true" ]; then
			break
		fi
	done

	local files=$(ls ${WORKING_DIR}/r${RELEASE}/ | wc -l)
	local uploads=$(${GITHUB_RELEASE_BIN} info --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} | grep -- '- artifact: ' | wc -l)
	test ${files} -eq ${uploads}

	popd
}

function clean() {
	( set +x ;\
		cd ${WORKING_DIR}/r${RELEASE}/ ;\
		ls -lht ;\
		echo ;\
		echo 'sha1sum *' ;\
		sha1sum * | xargs -n1 -i echo -e "\e[1;32m{}\e[0m" ;\
		rm -rf ${WORKING_DIR} ;\
	)
}

init_github
build_go
build_glog
build_http2
build_repo
if [ "x${TRAVIS_EVENT_TYPE}" == "xpush" ]; then
	build_repo_ex
	release_repo_ci
fi
clean
