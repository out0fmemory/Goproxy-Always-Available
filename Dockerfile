FROM progrium/busybox

WORKDIR /opt/goproxy-vps

RUN opkg-install curl
RUN echo tlsv1 >> ~/.curlrc

RUN mkdir -p /opt/goproxy-vps
RUN curl -Lk https://github.com/phuslu/goproxy/releases/download/goproxy/$(curl -Lks https://git.io/goproxy | grep -oE 'goproxy-vps_linux_amd64-r[0-9]+.tar.xz' | head -1) | xz -d | tar xvf - -C /opt/goproxy-vps

EXPOSE 443

ENTRYPOINT ["/opt/goproxy-vps/goproxy-vps"]
