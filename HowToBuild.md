#如何编译 GoProxy

GoProxy 对 golang 周边库做了一些修改。具体的改动请见，

1. https://github.com/phuslu/go
1. https://github.com/phuslu/net
1. https://github.com/phuslu/glog

所以编译需要从 golang 工具链开始编译, 以下步骤都假设你的工作目录位于 ~/workspace/goproxy/

- 保证系统安装了如下工具 awk/git/tar/bzip2/xz/7za/gcc/make/sha1sum/timeout/xargs，检查命令：
```bash
for CMD in curl awk git tar bzip2 xz 7za gcc sha1sum timeout xargs
do
	if ! type -p ${CMD}; then
		echo -e "\e[1;31mtool ${CMD} is not installed, abort.\e[0m"
		exit 1
	fi
done
```
- 编译 golang 工具链
```bash
export GOROOT_BOOTSTRAP=~/workspace/goproxy/goroot_bootstrap
export GOROOT=~/workspace/goproxy/go
export GOPATH=~/workspace/goproxy/gopath

cd ~/workspace/goproxy/

curl -k https://storage.googleapis.com/golang/go1.4.2.linux-amd64.tar.gz | tar xz
mv go goroot_bootstrap

git clone --depth 1 https://github.com/phuslu/go
(cd go/src && bash ./make.bash)

export PATH=$PATH:~/workspace/goproxy/go/bin
```
- 编译 goproxy
```bash
git clone https://github.com/phuslu/goproxy
cd goproxy
git checkout master

awk 'match($1, /"((github\.com|golang\.org|gopkg\.in)\/.+)"/) {if (!seen[$1]++) {gsub("\"", "", $1); print $1}}' $(find . -name "*.go") | xargs -n1 -i go get -v -u {}

go build -v
```
- 运行调试 goproxy
```bash
./goproxy -v=3
```
- 打包 goproxy
```bash
./make.bash
```
- 交叉编译+打包 goproxy
```bash
GOOS=windows GOARCH=amd64 ./make.bash
```
- 一键编译 GoProxy
```bash
bash -xe < <(curl -kL https://github.com/phuslu/goproxy/raw/master/assets/build/ci.sh)
```
