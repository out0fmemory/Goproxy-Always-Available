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
    local daemon_stderr=$(cat goproxy-vps.user.toml goproxy-vps.toml 2>/dev/null | awk -F= '/daemon_stderr/{gsub(/ /, "", $2); gsub(/"/, "", $2); print $2; exit}')
    local log_file=${daemon_stderr:-goproxy-vps.log}
    log_file=$(cd $(dirname ${log_file}); echo $(pwd -P)/$(basename ${log_file}))
    test $(ulimit -n) -lt 65535 && ulimit -n 65535
    nohup ./goproxy-vps >>${log_file} 2>&1 &
    local pid=$!
    echo -n "Starting ${PACKAGE_NAME}(${pid}): "
    sleep 1
    if (ps ax 2>/dev/null || ps) | grep "${pid} " >/dev/null 2>&1; then
        echo "OK"
    else
        echo "Failed"
    fi

    if test -f ${log_file}; then
        if test -d /etc/logrotate.d; then
            cat <<EOF > /etc/logrotate.d/goproxy-vps
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
    for pid in $( (ps ax 2>/dev/null || ps) | awk '/goproxy-vps(\s|$)/{print $1}')
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
    if ! command -v crontab >/dev/null ; then
        echo "ERROR: please install cron"
    fi
    (crontab -l | grep -v 'goproxy-vps.sh'; echo "*/1 * * * * pgrep goproxy-vps >/dev/null || $(pwd)/goproxy-vps.sh start") | crontab -
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

