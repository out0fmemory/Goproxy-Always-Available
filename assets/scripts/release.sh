#!/bin/bash

GOROOT=/go
GOPATH=/home/phuslu/GOPATH
PATH=$PATH:$GOROOT/bin:$GOPATH/bin

cd ${GOPATH}/src/github.com/phuslu/goproxy
git clean -df
git reset --hard

git branch -D bak
git checkout -b bak
git branch -D master
git fetch origin
git checkout master

REV=`git rev-list HEAD | wc -l | xargs`
NOTE=`git log --oneline | head -1`
REMOTE=`git remote -v | head -1 | awk '{print $2}'`

cd ${GOPATH}/src/github.com/phuslu/goproxy/goproxy
mkdir -p dist

PACKAGE_GOOS=windows PACKAGE_GOARCH=386 make && mv build/dist/goproxy* dist/ && make clean
PACKAGE_GOOS=windows PACKAGE_GOARCH=amd64 make && mv build/dist/goproxy* dist/ && make clean
PACKAGE_GOOS=linux PACKAGE_GOARCH=amd64 make && mv build/dist/goproxy* dist/ && make clean
PACKAGE_GOOS=linux PACKAGE_GOARCH=386 make && mv build/dist/goproxy* dist/ && make clean
PACKAGE_GOOS=linux PACKAGE_GOARCH=arm make && mv build/dist/goproxy* dist/ && make clean
PACKAGE_GOOS=darwin PACKAGE_GOARCH=amd64 make && mv build/dist/goproxy* dist/ && make clean
PACKAGE_GOOS=darwin PACKAGE_GOARCH=386 make && mv build/dist/goproxy* dist/ && make clean

export GITHUB_TOKEN=`cat ~/GITHUB_TOKEN`

github-release delete --user phuslu --repo goproxy --tag goproxy
github-release release --user phuslu --repo goproxy --tag goproxy --name "goproxy r${REV}" --description "r${REV}: ${NOTE}"
for f in `ls dist`; do
    github-release -v upload --user phuslu --repo goproxy --tag goproxy --name $f --file dist/$f
done
