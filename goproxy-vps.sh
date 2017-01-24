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
    setcap cap_net_bind_service=+ep goproxy-vps
    su ${SUDO_USER} -c 'nohup ./goproxy-vps >./goproxy-vps.log 2>&1 &'
    echo "${PACKAGE_NAME}"
}

stop() {
    echo -n "Stopping ${PACKAGE_DESC}: "
    killall goproxy-vps && echo "${PACKAGE_NAME}" || echo "${PACKAGE_NAME}"
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
    echo "ERROR: Please run as root"
    exit 1
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
