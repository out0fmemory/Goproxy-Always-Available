FROM alpine:latest

RUN apk add --no-cache curl xz tar && \
	mkdir -p /opt/ && \
	goproxy_dist=$(curl -Ls https://git.io/goproxy | grep -oE 'goproxy_linux_amd64-r[0-9]+.tar.xz' | head -1) && \
	curl -L https://github.com/phuslu/goproxy/releases/download/goproxy/${goproxy_dist} | xz -d | tar xvf - -C /opt/ && \
	curl -Lf https://github.com/phuslu/cmdhere/raw/master/gae.user.json >/opt/goproxy/gae.user.json && \
	echo '{"RegionFilters":{"Enabled": true}}' >/opt/goproxy/autoproxy.user.json && \
	echo '{"Default":{"Address": ":10"}}' >/opt/goproxy/httpproxy.user.json && \
	sh -c "GOPROXY_WAIT_SECONDS=3 /opt/goproxy/goproxy"

EXPOSE 10

ENTRYPOINT ["/opt/goproxy/goproxy", "-v=1"]
