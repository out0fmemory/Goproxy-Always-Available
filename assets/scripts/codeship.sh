export GITHUB_USER=${GITHUB_USER}
export GITHUB_PASS=${GITHUB_PASS}
export GITHUB_TOKEN=${GITHUB_TOKEN}
export GITHUB_REPO=${GITHUB_REPO}
export GITHUB_CI_REPO=${GITHUB_CI_REPO}
export GOROOT=~/go
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
curl -k https://storage.googleapis.com/golang/go1.6.linux-amd64.tar.gz | tar xz -C ~/
cat /etc/issue && uname -a && echo && go version && echo && go env && echo && env
git clone --branch "master" https://github.com/${GITHUB_USER}/${GITHUB_REPO} ~/${GITHUB_REPO}
git clone --branch "master" https://${GITHUB_USER}:${GITHUB_PASS}@github.com/${GITHUB_USER}/${GITHUB_CI_REPO} ~/${GITHUB_CI_REPO}
cd ~/${GITHUB_REPO}
export RELEASE=`git rev-list HEAD|wc -l|xargs`
export NOTE=`git log --oneline | head -1 | awk -v GITHUB_USER=${GITHUB_USER} -v GITHUB_REPO=${GITHUB_REPO} '{$1="[\`"$1"\`](https://github.com/"GITHUB_USER"/"GITHUB_REPO"/commit/"$1")";print}'`
mkdir ~/${GITHUB_CI_REPO}/r${RELEASE}
make clean && make GOOS=linux GOARCH=amd64 && cp -r build/dist/* ~/${GITHUB_CI_REPO}/r${RELEASE}
make clean && make GOOS=linux GOARCH=386 && cp -r build/dist/* ~/${GITHUB_CI_REPO}/r${RELEASE}
make clean && make GOOS=linux GOARCH=arm && cp -r build/dist/* ~/${GITHUB_CI_REPO}/r${RELEASE}
make clean && make GOOS=windows GOARCH=386  && cp -r build/dist/* ~/${GITHUB_CI_REPO}/r${RELEASE}
make clean && make GOOS=windows GOARCH=amd64  && cp -r build/dist/* ~/${GITHUB_CI_REPO}/r${RELEASE}
make clean && make GOOS=darwin GOARCH=386  && cp -r build/dist/* ~/${GITHUB_CI_REPO}/r${RELEASE}
make clean && make GOOS=darwin GOARCH=amd64  && cp -r build/dist/* ~/${GITHUB_CI_REPO}/r${RELEASE}
ls -lht ~/${GITHUB_CI_REPO}/r${RELEASE}/*
go get github.com/aktau/github-release
cd ~/${GITHUB_CI_REPO}/
git config --global user.name ${GITHUB_USER}
git config --global user.email "${GITHUB_USER}@noreply.github.com"
git commit --allow-empty -m "release"
git tag -d r${RELEASE} || true
git tag r${RELEASE}
git push -f origin r${RELEASE}
github-release delete --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} || true
sleep 1
github-release release --pre-release --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} --name "${GITHUB_REPO} r${RELEASE}" --description "r${RELEASE}: ${NOTE}"
for f in `ls *`; do github-release -v upload --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} --name $f --file r${RELEASE}/$f; done
