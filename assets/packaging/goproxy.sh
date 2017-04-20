#!/bin/sh

### BEGIN INIT INFO
# Provides:       goproxy
# Required-Start: $network
# Required-Stop:
# Should-Start:
# Should-Stop:
# Default-Start: 2 3 4 5
# Default-Stop:  0 1 6
# Short-Description: start and stop daemon script
# Description: a daemon script
### END INIT INFO

set -e

EXECUTABLE=$(basename "${0}" | sed -r 's/\.[^.]+$//')
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin:${PATH}
SUDO=$(test $(id -u) = 0 || echo sudo)

linkpath=$(ls -l "$0" | sed "s/.*->\s*//")
cd "$(dirname "$0")" && test -f "$linkpath" && cd "$(dirname "$linkpath")" || true

start() {
    local log_file=${EXECUTABLE}.log
    log_file=$(cd $(dirname ${log_file}); echo $(pwd -P)/$(basename ${log_file}))

    test $(ulimit -n) -lt 65535 && ulimit -n 65535
    if command -v nohup >/dev/null ; then
        nohup ./${EXECUTABLE} >>${log_file} 2>&1 &
    else
        echo "please install nohup"
        exit 1
    fi

    local pid=$!
    echo -n "Starting ${EXECUTABLE}(${pid}): "
    sleep 1
    if ls /proc/${pid}/cmdline >/dev/null 2>&1; then
        echo "OK"
    else
        echo "Failed"
    fi

    if test -f ${log_file}; then
        if test -d /etc/logrotate.d; then
            cat <<EOF > /etc/logrotate.d/${EXECUTABLE}
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
    for pid in $( (ps ax 2>/dev/null || ps) | awk "/${EXECUTABLE}(\\s|\$)/{print \$1}")
    do
        local exe=$(ls -l /proc/${pid}/exe 2>/dev/null | sed "s/.*->\s*//" | sed 's/\s*(deleted)\s*//')
        local cwd=$(ls -l /proc/${pid}/cwd 2>/dev/null | sed "s/.*->\s*//" | sed 's/\s*(deleted)\s*//')
        if test "$(basename "$exe")" = "${EXECUTABLE}" -a "$cwd" = "$(pwd)"; then
            echo -n "Stopping ${EXECUTABLE}(${pid}): "
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
    ln -sf $(pwd)/${EXECUTABLE}.sh /etc/init.d/${EXECUTABLE}
    if command -v update-rc.d >/dev/null ; then
        update-rc.d ${EXECUTABLE} defaults
    elif command -v chkconfig >/dev/null ; then
        chkconfig ${EXECUTABLE} on
    elif command -v systemctl >/dev/null ; then
        systemctl enable ${EXECUTABLE}
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
