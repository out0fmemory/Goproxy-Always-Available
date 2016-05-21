#!/bin/sh
#
# goproxy init script
#

### BEGIN INIT INFO
# Provides:          goproxy
# Required-Start:    $syslog
# Required-Stop:     $syslog
# Should-Start:      $local_fs
# Should-Stop:       $local_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Monitor for goproxy activity
# Description:       goproxy is a go proxy
### END INIT INFO

# **NOTE** bash will exit immediately if any command exits with non-zero.
set -e

PACKAGE_NAME=goproxy
PACKAGE_DESC="a go proxy"
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin:${PATH}

start() {
    echo -n "Starting ${PACKAGE_DESC}: "
    nohup ./goproxy -v=2 -logtostderr=0 -log_dir=/var/log/ &
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

if [ "$(id -u)" != "0" ]; then
    echo "please use sudo to run ${PACKAGE_NAME}"
    exit 0
fi

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

exit 0
