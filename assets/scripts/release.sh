#!/bin/bash

GOROOT=$HOME/go
GOPATH=$HOME/gopath
PATH=$PATH:$GOROOT/bin:$GOPATH/bin
SOURCEDIR=$HOME/goproxy
DISTDIR=${SOURCEDIR}/dist

cd ${SOURCEDIR}
git clean -df
git reset --hard
mkdir -p ${DISTDIR}

git branch -D bak
git checkout -b bak
git branch -D master
git fetch origin
git checkout master

REV=`git rev-list HEAD | wc -l | xargs`
NOTE=`git log --oneline | head -1`
REMOTE=`git remote -v | head -1 | awk '{print $2}'`

# cd ${SOURCEDIR}/fetchserver/vps
# GOOS=linux GOARCH=amd64 make && mv build/dist/govps* ${DISTDIR}/ && make clean

cd ${SOURCEDIR}
GOOS=windows GOARCH=386 make && mv build/dist/goproxy* ${DISTDIR}/ && make clean
GOOS=windows GOARCH=amd64 make && mv build/dist/goproxy* ${DISTDIR}/ && make clean
GOOS=linux GOARCH=amd64 make && mv build/dist/goproxy* ${DISTDIR}/ && make clean
GOOS=linux GOARCH=386 make && mv build/dist/goproxy* ${DISTDIR}/ && make clean
GOOS=linux GOARCH=arm make && mv build/dist/goproxy* ${DISTDIR}/ && make clean
GOOS=darwin GOARCH=amd64 make && mv build/dist/goproxy* ${DISTDIR}/ && make clean
GOOS=darwin GOARCH=386 make && mv build/dist/goproxy* ${DISTDIR}/ && make clean

export GITHUB_TOKEN=`cat ~/GITHUB_TOKEN`

github-release delete --user phuslu --repo goproxy --tag goproxy
github-release release --user phuslu --repo goproxy --tag goproxy --name "goproxy r${REV}" --description "r${REV}: ${NOTE}"
for f in `ls ${DISTDIR}`; do
    github-release -v upload --user phuslu --repo goproxy --tag goproxy --name $f --file ${DISTDIR}/$f
done
