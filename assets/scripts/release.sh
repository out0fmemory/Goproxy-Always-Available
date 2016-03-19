#!/bin/bash

if [ -z "$GITHUB_TOKEN" ]; then
	GITHUB_TOKEN=`cat ~/GITHUB_TOKEN`
fi
export GITHUB_TOKEN=$GITHUB_TOKEN

GOROOT=$HOME/go
GOPATH=$HOME/gopath
PATH=$PATH:$GOROOT/bin:$GOPATH/bin
SOURCEDIR=$HOME/goproxy
DISTDIR=${SOURCEDIR}/dist

cd ${SOURCEDIR}
git fetch origin
git reset --hard origin/master
git clean -dfx

RELEASE=`git rev-list HEAD | wc -l | xargs`
NOTE=`git log --oneline | head -1`

mkdir -p ${DISTDIR}
cd ${SOURCEDIR}
for OSARCH in windows/amd64 windows/386 linux/amd64 linux/386 linux/arm darwin/amd64 darwin/386; do
	GOOS=${OSARCH%/*} GOARCH=${OSARCH#*/} make
	mv build/dist/goproxy* ${DISTDIR}/
	make clean
done

github-release delete --user phuslu --repo goproxy --tag goproxy
github-release release --user phuslu --repo goproxy --tag goproxy --name "goproxy r${RELEASE}" --description "r${RELEASE}: ${NOTE}"
for f in `ls ${DISTDIR}`; do
    github-release -v upload --user phuslu --repo goproxy --tag goproxy --name $f --file ${DISTDIR}/$f
done
