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
    mkdir -p /var/log/goproxy
    nohup ./goproxy -v=2 -logtostderr=0 -log_dir=/var/log/goproxy &
    echo "${PACKAGE_NAME}."
}

stop() {
    echo -n "Stopping ${PACKAGE_DESC}: "
    killall goproxy >/dev/null 2>&1 || true
    echo "${PACKAGE_NAME}."
}

restart() {
    stop || true
    sleep 1
    start
}

usage() {
    N=$(basename "$0")
    echo "Usage: [sudo] $N {start|stop|restart}" >&2
    exit 1
}

# `readlink -f` won't work on Mac, this hack should work on all systems.
cd $(python -c "import os; print(os.path.dirname(os.path.realpath('$0')))")

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
