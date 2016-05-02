#如何编译 GoProxy

以下步骤都假设你的工作目录位于 ~/workspace/goproxy/

- 保证系统安装了如下工具 awk/git/tar/bzip2/xz/7za/gcc，检查命令：
```bash
for CMD in curl awk git tar bzip2 xz 7za gcc; do
	if ! $(which ${CMD} >/dev/null 2>&1); then
		echo "tool ${CMD} is not installed, abort."
	fi
done
```
- 编译 golang 工具链
```bash
export GOROOT_BOOTSTRAP=~/workspace/goproxy/go1.6
export GOROOT=~/workspace/goproxy/go
export GOPATH=~/workspace/goproxy/gopath

cd ~/workspace/goproxy/

curl -k https://storage.googleapis.com/golang/go1.6.linux-amd64.tar.gz | tar xz
mv go go1.6

git clone https://github.com/phuslu/go
(go/src && bash ./make.bash)
```
- 编译 goproxy
```bash
git clone https://github.com/phuslu/goproxy
cd goproxy

awk 'match($1, /"((github\.com|golang\.org|gopkg\.in)\/.+)"/) {if (!seen[$1]++) {gsub("\"", "", $1); print $1}}' $(find . -name "*.go") | xargs -n1 -i go get -v -u {}

go build -v
```
- 运行调试 goproxy
```bash
./goproxy -v=2
```
- 打包 goproxy
```bash
make
```
- 交叉编译+打包 goproxy
```bash
make GOOS=windows GOARCH=amd64
```
