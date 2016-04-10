#!/bin/bash

set -x
set -e

export GITHUB_USER=${GITHUB_USER:-phuslu}
export GITHUB_REPO=goproxy
export GITHUB_CI_REPO=goproxy-ci
export GITHUB_TAG=${GITHUB_TAG}
export GITHUB_TOKEN=${GITHUB_TOKEN:-$(cat ~/GITHUB_TOKEN)}
export GITHUB_CI_REPO_RELEASE_INFO_TXT=github-${GITHUB_USER}-${GITHUB_CI_REPO}-${GITHUB_TAG}-release-info.txt
if [ -z "$GITHUB_TOKEN" ]; then
	echo Please set GITHUB_TOKEN envar
	exit 1
fi

trap 'rm -rf $HOME/tmp.*.${GITHUB_REPO}; exit' SIGINT SIGQUIT SIGTERM

export WORKING_DIR=${HOME}/tmp.$$.${GITHUB_REPO}
mkdir -p $WORKING_DIR
pushd ${WORKING_DIR}

curl -L https://github.com/aktau/github-release/releases/download/v0.6.2/linux-amd64-github-release.tar.bz2 | tar xjp -C .
export PATH=${WORKING_DIR}/bin/linux/amd64:$PATH

if [ -z "${GITHUB_TAG}" ]; then
export GITHUB_TAG=$(github-release info -u ${GITHUB_USER} -r ${GITHUB_CI_REPO} | head | grep -oP '\- \Kr\d+' | head -1)
fi

github-release info -u ${GITHUB_USER} -r ${GITHUB_CI_REPO} -t ${GITHUB_TAG} > ${GITHUB_CI_REPO_RELEASE_INFO_TXT}
export RELEASE_NAME=`cat ${GITHUB_CI_REPO_RELEASE_INFO_TXT} | grep -oP "name: '\K.+?'," | sed 's/..$//'`
export RELEASE_NOTE=`cat ${GITHUB_CI_REPO_RELEASE_INFO_TXT} | grep -oP "description: '\K.+?'," | sed 's/..$//'`
export RELEASE_FILES=`cat ${GITHUB_CI_REPO_RELEASE_INFO_TXT} | grep -oP 'artifact: \K\S+\.(7z|zip|gz|bz2|xz|tar)'`

for FILE in ${RELEASE_FILES}; do
    github-release download --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag ${GITHUB_TAG} --name "$FILE"
done

github-release delete --user ${GITHUB_USER} --repo ${GITHUB_REPO} --tag ${GITHUB_REPO}
github-release release --user ${GITHUB_USER} --repo ${GITHUB_REPO} --tag ${GITHUB_REPO} --name "${RELEASE_NAME}" --description "${RELEASE_NOTE}"
for FILE in ${RELEASE_FILES}; do
    github-release upload --user ${GITHUB_USER} --repo ${GITHUB_REPO} --tag ${GITHUB_REPO} --name "${FILE}" --file "${FILE}"
done

popd
rm -rf ${WORKING_DIR}
