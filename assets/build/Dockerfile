FROM gliderlabs/alpine:latest

RUN apk add --update curl && rm -rf /var/cache/apk/*

WORKDIR /opt/goproxy

RUN mkdir -p /opt/goproxy
RUN curl -Lk https://github.com/phuslu/goproxy/releases/download/goproxy/$(curl -Lks https://git.io/goproxy | grep -oE 'goproxy_linux_amd64-r[0-9]+.tar.xz' | head -1) | xz -d | tar xvf - -C /opt/goproxy

ADD https://github.com/phuslu/goproxy/raw/master/gae.user.json /opt/goproxy/gae.user.json

RUN sh -c "GOPROXY_WAIT_SECONDS=5 /opt/goproxy/goproxy"

EXPOSE 8087

ENTRYPOINT ["/opt/goproxy/goproxy"]
