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
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin:${PATH}
SUDO=$(test $(id -u) = 0 || echo sudo)

linkpath=$(ls -l "$0" | sed "s/.*->\s*//")
cd "$(dirname "$0")" && test -f "$linkpath" && cd "$(dirname "$linkpath")" || true

start() {
    local log_file=$(cat goproxy-vps.user.toml goproxy-vps.toml 2>/dev/null | awk -F= '/daemon_stderr\s*=/{gsub(/ /, "", $2); gsub(/"/, "", $2); print $2; exit}')
    nohup ./goproxy-vps >>${log_file:-goproxy-vps.log} 2>&1 &
    local pid=$!
    echo -n "Starting ${PACKAGE_NAME}(${pid}): "
    sleep 1
    if ps ax | grep "^${pid}" >/dev/null 2>&1; then
        echo "OK"
    else
        echo "Failed"
    fi
}

stop() {
    for pid in $(ps ax | grep -v grep | grep ./goproxy-vps | awk '{print $1}')
    do
        local exe=$(ls -l /proc/${pid}/exe 2>/dev/null | sed "s/.*->\s*//" | sed 's/\s*(deleted)\s*//')
        local cwd=$(ls -l /proc/${pid}/cwd 2>/dev/null | sed "s/.*->\s*//" | sed 's/\s*(deleted)\s*//')
        if test "$(basename "$exe")" = "goproxy-vps" -a "$cwd" = "$(pwd)"; then
            echo -n "Stopping ${PACKAGE_NAME}(${pid}): "
            if kill $pid; then
                echo "OK"
            else
                echo "Failed"
            fi
        fi
    done
}

restart() {
    stop
    sleep 1
    start
}

autostart() {
    ln -sf $(pwd)/goproxy-vps.sh /etc/init.d/goproxy-vps
    if command -v update-rc.d >/dev/null ; then
        update-rc.d goproxy-vps defaults
    elif command -v chkconfig >/dev/null ; then
        chkconfig goproxy-vps on
    fi
}

usage() {
    echo "Usage: [sudo] $(basename "$0") {start|stop|restart|autostart}" >&2
    exit 1
}

if [ -n "${SUDO}" ]; then
    echo "ERROR: Please run as root"
    exit 1
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
    autostart)
        autostart
        ;;
    *)
        usage
        ;;
esac

exit $?

