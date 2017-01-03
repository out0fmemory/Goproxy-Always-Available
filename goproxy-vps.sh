#!/bin/sh
#
#       /etc/rc.d/init.d/goproxy-vps
#
#       a go proxy vps
#
# chkconfig:   2345 95 05
# description: a go proxy vps

### BEGIN INIT INFO
# Provides:       goproxy-vps
# Required-Start: $network
# Required-Stop:
# Should-Start:
# Should-Stop:
# Default-Start: 2 3 4 5
# Default-Stop:  0 1 6
# Short-Description: start and stop goproxy-vps
# Description: a go proxy vps
### END INIT INFO

set -e

PACKAGE_NAME=goproxy-vps
PACKAGE_DESC="a go proxy vps"
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin:${PATH}
SUDO=$(test $(id -u) = 0 || echo sudo)
DOAMIN_FILE=acme_domain.txt

start() {
    echo -n "Starting ${PACKAGE_DESC}: "
    test -f ${DOAMIN_FILE} || echo "Please put your vps domain name to ./${DOAMIN_FILE}"
    local acmedomain=${DOAMIN:-$(cat ${DOAMIN_FILE})}
    local extra_args=$(cat ./extra-args.txt 2>/dev/null | tr '\n' ' ')
    local log_dir=$(test -d "/var/log" && echo "/var/log/goproxy" || echo "$(pwd)/logs")
    mkdir -p ${log_dir}
    nohup ./goproxy-vps -addr=:443 -acmedomain=${acmedomain} -v=2 -logtostderr=0 -log_dir=${log_dir} -tls12 ${extra_args} >/dev/null 2>&1 &
    echo -e "\e[1;32m${PACKAGE_NAME}\e[0m\n"
}

stop() {
    echo -n "Stopping ${PACKAGE_DESC}: "
    killall goproxy-vps && echo -e "\e[1;32m${PACKAGE_NAME}\e[0m\n" || echo -e "\e[1;31m${PACKAGE_NAME}\e[0m\n"
}

restart() {
    stop
    sleep 1
    start
}

usage() {
    echo "Usage: [sudo] $(basename "$0") {start|stop|restart}" >&2
    exit 1
}

if [ -n "${SUDO}" ]; then
    echo -e "\e[1;32mPlease run as root\e[0m\n"
fi

linkpath=$(ls -l "$0" | sed "s/.*->\s*//")
cd "$(dirname "$0")" && test -f "$linkpath" && cd "$(dirname "$linkpath")" || true

case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    *)
        usage
        ;;
esac

exit $?
