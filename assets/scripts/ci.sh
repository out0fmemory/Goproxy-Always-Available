export GITHUB_USER=${GITHUB_USER:-phuslu}
export GITHUB_REPO=${GITHUB_REPO:-goproxy}
export GITHUB_CI_REPO=${GITHUB_CI_REPO:-goproxy-ci}
export GITHUB_PASS=${GITHUB_PASS:-$(cat ~/GITHUB_PASS)}
export GITHUB_TOKEN=${GITHUB_TOKEN:-$(cat ~/GITHUB_TOKEN)}
export GITHUB_COMMIT_ID=${COMMIT_ID:-master}
export WORKING_DIR=$HOME/tmp.${RANDOM:-$$}.${GITHUB_REPO}
export GOROOT_BOOTSTRAP=${WORKING_DIR}/go1.6
export GOROOT=${WORKING_DIR}/go
export GOPATH=${WORKING_DIR}/gopath
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
mkdir -p ${WORKING_DIR}
cd ${WORKING_DIR}
curl -k https://storage.googleapis.com/golang/go1.6.linux-amd64.tar.gz | tar xz
mv go go1.6
git clone https://github.com/phuslu/go
(cd go && git remote add -f upstream https://github.com/golang/go && git rebase upstream/master)
(cd go/src/ && bash ./make.bash)
cat /etc/issue && uname -a && echo && go version && echo && go env && echo && env
git clone --branch "master" https://github.com/${GITHUB_USER}/${GITHUB_REPO} ${GITHUB_REPO}
git clone --branch "master" https://${GITHUB_USER}:${GITHUB_PASS}@github.com/${GITHUB_USER}/${GITHUB_CI_REPO} ${GITHUB_CI_REPO}
curl -L https://github.com/aktau/github-release/releases/download/v0.6.2/linux-amd64-github-release.tar.bz2 | tar xjpv | xargs -n1 -i mv -f {} $GOROOT/bin/
cd ${GITHUB_REPO}
git checkout -f ${GITHUB_COMMIT_ID}
export RELEASE=`git rev-list HEAD|wc -l|xargs`
#export LATEST_RELEASE=`github-release info -u ${GITHUB_USER} -r ${GITHUB_CI_REPO} | head -5 | grep -oP "\- r\K\d+" | head -1`
#export NCOMMITS=$([[ $((${RELEASE} - ${LATEST_RELEASE})) -gt 1 ]] && echo $((${RELEASE} - ${LATEST_RELEASE})) || echo 1)
export NOTE=`git log --oneline -1 | awk -v GITHUB_USER=${GITHUB_USER} -v GITHUB_REPO=${GITHUB_REPO} '{$1="[\`"$1"\`](https://github.com/"GITHUB_USER"/"GITHUB_REPO"/commit/"$1")";print}'`
mkdir ${WORKING_DIR}/r${RELEASE}
awk 'match($1, /"((github\.com|golang\.org|gopkg\.in)\/.+)"/) {if (!seen[$1]++) {gsub("\"", "", $1); print $1}}' `find . -name "*.go"` | xargs -n1 -i go get -v {}
make clean && make GOOS=linux GOARCH=amd64 && cp -r build/dist/* ${WORKING_DIR}/r${RELEASE}
make clean && make GOOS=linux GOARCH=386 && cp -r build/dist/* ${WORKING_DIR}/r${RELEASE}
make clean && make GOOS=linux GOARCH=arm && cp -r build/dist/* ${WORKING_DIR}/r${RELEASE}
make clean && make GOOS=windows GOARCH=386  && cp -r build/dist/* ${WORKING_DIR}/r${RELEASE}
make clean && make GOOS=windows GOARCH=amd64  && cp -r build/dist/* ${WORKING_DIR}/r${RELEASE}
make clean && make GOOS=darwin GOARCH=386  && cp -r build/dist/* ${WORKING_DIR}/r${RELEASE}
make clean && make GOOS=darwin GOARCH=amd64  && cp -r build/dist/* ${WORKING_DIR}/r${RELEASE}
ls -lht ${WORKING_DIR}/r${RELEASE}/*
cd ${WORKING_DIR}/${GITHUB_CI_REPO}/
git config user.name ${GITHUB_USER}
git config user.email "${GITHUB_USER}@noreply.github.com"
git commit --allow-empty -m "release"
git tag -d r${RELEASE} || true
git tag r${RELEASE}
git push -f origin r${RELEASE}
cd ${WORKING_DIR}/r${RELEASE}/
github-release delete --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} || true
sleep 1
github-release release --pre-release --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} --name "${GITHUB_REPO} r${RELEASE}" --description "r${RELEASE}: ${NOTE}"
for FILE in *; do github-release upload --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag r${RELEASE} --name ${FILE} --file ${FILE}; done
ls -lht && cd && rm -rf $HOME/tmp.*.${GITHUB_REPO}
