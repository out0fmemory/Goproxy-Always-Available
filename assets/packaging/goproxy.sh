#!/bin/sh
#
#       /etc/rc.d/init.d/goproxy
#
#       a go proxy
#
# chkconfig:   2345 95 05
# description: a go proxy

### BEGIN INIT INFO
# Provides:       goproxy
# Required-Start: $network
# Required-Stop:
# Should-Start:
# Should-Stop:
# Default-Start: 2 3 4 5
# Default-Stop:  0 1 6
# Short-Description: start and stop goproxy
# Description: a go proxy
### END INIT INFO

set -e

PACKAGE_NAME=goproxy
PACKAGE_DESC="a go proxy"
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin:${PATH}

start() {
    echo -n "Starting ${PACKAGE_DESC}: "
    local log_dir=$(test -d "/var/log" && echo "/var/log/${PACKAGE_NAME}" || echo "$(pwd)/logs")
    mkdir -p ${log_dir}
    nohup ./goproxy -v=2 -logtostderr=0 -log_dir=${log_dir} >/dev/null 2>&1 &
    echo "${PACKAGE_NAME}."
}

stop() {
    echo -n "Stopping ${PACKAGE_DESC}: "
    killall goproxy
    echo "${PACKAGE_NAME}."
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

if readlink --help | grep -q -w -- '-f'; then
    cd "$(dirname "$(readlink -f "$0")")"
else
    cd "$(python -c "import os; print(os.path.dirname(os.path.realpath('$0')))")"
fi

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
