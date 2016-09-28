FROM alpine:latest

RUN apk add --no-cache curl bzip2 tar openssl && \
	mkdir -p /opt/goproxy-vps && \
	goproxy_vps_dist=$(curl -Lks https://git.io/goproxy | grep -oE 'goproxy-vps_linux_amd64-r[0-9]+.tar.xz' | head -1) && \
	curl -L https://github.com/phuslu/goproxy/releases/download/goproxy/${goproxy_vps_dist} | bzip2 -d | tar xvf - -C /opt/goproxy-vps

ENTRYPOINT ["/opt/goproxy-vps/goproxy-vps"]
