FROM alpine:latest

RUN apk add --update curl xz tar && rm -rf /var/cache/apk/* && \
	mkdir -p /opt/ && \
	goproxy_dist=$(curl -Ls https://git.io/goproxy | grep -oE 'goproxy_linux_amd64-r[0-9]+.tar.xz' | head -1) && \
	curl -L https://github.com/phuslu/goproxy/releases/download/goproxy/${goproxy_dist} | xz -d | tar xvf - -C /opt/ && \
	curl -Lf https://github.com/phuslu/cmdhere/raw/master/gae.user.json >/opt/goproxy/gae.user.json && \
	echo '{"Default":{"Address": ":8087"}}' >/opt/goproxy/httpproxy.user.json && \
	sh -c "GOPROXY_WAIT_SECONDS=5 /opt/goproxy/goproxy"

EXPOSE 8087

ENTRYPOINT ["/opt/goproxy/goproxy"]
