FROM alpine:latest

WORKDIR /opt/goproxy-vps

RUN apk add --no-cache curl bzip2 tar && \
	mkdir -p /opt/goproxy-vps && \
	goproxy_vps_dist=$(curl -Lks https://git.io/goproxy | grep -oE 'goproxy-vps_linux_amd64-r[0-9]+.tar.bz2' | head -1) && \
	curl -L https://github.com/phuslu/goproxy/releases/download/goproxy/${goproxy_vps_dist} | bzip2 -d | tar xvf - -C /opt/goproxy-vps

ADD goproxy-vps.crt goproxy-vps.key ./

EXPOSE 443

ENTRYPOINT ["/opt/goproxy-vps/goproxy-vps"]
