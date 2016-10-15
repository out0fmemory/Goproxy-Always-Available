FROM alpine:latest

RUN apk add --no-cache curl xz tar openssl && \
	mkdir -p /opt/goproxy-vps && \
	goproxy_vps_dist=$(curl -Lks https://git.io/goproxy | grep -oE 'goproxy-vps_linux_amd64-r[0-9]+.tar.xz' | head -1) && \
	curl -L https://github.com/phuslu/goproxy/releases/download/goproxy/${goproxy_vps_dist} | xz -d | tar xvf - -C /opt/goproxy-vps

#see https://hub.docker.com/r/phuslu/goproxy-vps/
