#!/bin/bash

set -ex

export GITHUB_USER=${GITHUB_USER:-phuslu}
export GITHUB_REPO=goproxy
export GITHUB_CI_REPO=goproxy-ci
export GITHUB_TAG=${GITHUB_TAG}
export GITHUB_TOKEN=${GITHUB_TOKEN:-$(cat ~/GITHUB_TOKEN)}
export GITHUB_CI_REPO_RELEASE_INFO_TXT=github-ci-release-info.txt

if [ -z "$GITHUB_TOKEN" ]; then
	echo Please set GITHUB_TOKEN envar
	exit 1
fi

trap 'rm -rf $HOME/tmp.*.${GITHUB_REPO}; exit' SIGINT SIGQUIT SIGTERM

WORKING_DIR=${HOME}/tmp.$$.${GITHUB_REPO}
mkdir -p $WORKING_DIR
pushd ${WORKING_DIR}

GITHUB_RELEASE_URL=https://github.com/aktau/github-release/releases/download/v0.6.2/linux-amd64-github-release.tar.bz2
GITHUB_RELEASE_BIN=$(pwd)/$(curl -L ${GITHUB_RELEASE_URL} | tar xjpv | head -1)

if [ -z "${GITHUB_TAG}" ]; then
GITHUB_TAG=$(${GITHUB_RELEASE_BIN} info -u ${GITHUB_USER} -r ${GITHUB_CI_REPO} | head | grep -oP '\- \Kr\d+' | head -1)
fi

${GITHUB_RELEASE_BIN} info -u ${GITHUB_USER} -r ${GITHUB_CI_REPO} -t ${GITHUB_TAG} > ${GITHUB_CI_REPO_RELEASE_INFO_TXT}
export RELEASE_NAME=`cat ${GITHUB_CI_REPO_RELEASE_INFO_TXT} | grep -oP "name: '\K.+?'," | sed 's/..$//'`
export RELEASE_NOTE=`cat ${GITHUB_CI_REPO_RELEASE_INFO_TXT} | grep -oP "description: '\K.+?'," | sed 's/..$//'`
export RELEASE_FILES=`cat ${GITHUB_CI_REPO_RELEASE_INFO_TXT} | grep -oP 'artifact: \K\S+\.(7z|zip|gz|bz2|xz|tar)'`

for FILE in ${RELEASE_FILES}
do
    echo Downloading ${FILE} from https://github.com/${GITHUB_USER}/${GITHUB_CI_REPO}.git#${GITHUB_TAG}
    ${GITHUB_RELEASE_BIN} download --user ${GITHUB_USER} --repo ${GITHUB_CI_REPO} --tag ${GITHUB_TAG} --name "$FILE"
done

${GITHUB_RELEASE_BIN} delete --user ${GITHUB_USER} --repo ${GITHUB_REPO} --tag ${GITHUB_REPO} || true

git init
git config user.name "${GITHUB_USER}"
git config user.email "${GITHUB_USER}@noreply.github.com"
git remote add origin https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${GITHUB_USER}/${GITHUB_REPO}
git fetch origin ${GITHUB_REPO}
git checkout -b release FETCH_HEAD
git commit --amend --no-edit --allow-empty
git tag ${GITHUB_REPO}
git push -f origin ${GITHUB_REPO}
rm -rf .git

${GITHUB_RELEASE_BIN} release --user ${GITHUB_USER} --repo ${GITHUB_REPO} --tag ${GITHUB_REPO} --name "${RELEASE_NAME}" --description "${RELEASE_NOTE}"

for FILE in ${RELEASE_FILES}
do
    echo Uploading ${FILE} to https://github.com/${GITHUB_USER}/${GITHUB_REPO}.git#${GITHUB_TAG}
    ${GITHUB_RELEASE_BIN} upload --user ${GITHUB_USER} --repo ${GITHUB_REPO} --tag ${GITHUB_REPO} --name "${FILE}" --file "${FILE}"
done

popd
rm -rf ${WORKING_DIR}
