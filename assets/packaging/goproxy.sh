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
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin:${PATH}
SUDO=$(test $(id -u) = 0 || echo sudo)

linkpath=$(ls -l "$0" | sed "s/.*->\s*//")
cd "$(dirname "$0")" && test -f "$linkpath" && cd "$(dirname "$linkpath")" || true

start() {
    local log_file=./goproxy.log
    if command -v nohup >/dev/null ; then
        nohup ./goproxy >>${log_file} 2>&1 &
        local pid=$!
    elif busybox start-stop-daemon --help 2>/dev/null ; then
        busybox start-stop-daemon -S -b -x ./goproxy -- -v=2 -logtostderr=0 -log_dir=./logs
        local pid=$!
    else
        echo "please install nohup"
        exit 1
    fi

    echo -n "Starting ${PACKAGE_NAME}(${pid}): "

    sleep 1
    if ps ax | grep "^${pid} " >/dev/null 2>&1; then
        echo "OK"
    else
        echo "Failed"
    fi

    if test -f ${log_file}; then
        if test -d /etc/logrotate.d; then
            cat <<EOF > /etc/logrotate.d/goproxy
${log_file} {
    daily
    copytruncate
    missingok
    notifempty
    rotate 2
    compress
}
EOF
        fi
    fi
}

stop() {
    for pid in $(ps ax | awk '/goproxy(\s|$)/{print $1}')
    do
        local exe=$(ls -l /proc/${pid}/exe 2>/dev/null | sed "s/.*->\s*//" | sed 's/\s*(deleted)\s*//')
        local cwd=$(ls -l /proc/${pid}/cwd 2>/dev/null | sed "s/.*->\s*//" | sed 's/\s*(deleted)\s*//')
        if test "$(basename "$exe")" = "goproxy" -a "$cwd" = "$(pwd)"; then
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
    ln -sf $(pwd)/goproxy.sh /etc/init.d/goproxy
    if command -v update-rc.d >/dev/null ; then
        update-rc.d goproxy defaults
    elif command -v chkconfig >/dev/null ; then
        chkconfig goproxy on
    fi
}

usage() {
    echo "Usage: [sudo] $(basename "$0") {start|stop|restart}" >&2
    exit 1
}

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

