#!/bin/sh
#
### BEGIN INIT INFO
# Provides:          imunify-nsq
# Required-Start:    $syslog
# Required-Stop:     $syslog
# Should-Start:      $network
# Should-Stop:       $network
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: IPv4/IPv6 iptables packets processor.
# Description:       imunify-nsq message queue service
### END INIT INFO

# lock script execution as recommended by flock manual page
[ "${FLOCKER}" != "$0" ] && exec env FLOCKER="$0" flock -en "$0" "$0" "$@" || :

# Source function library.
. /etc/rc.d/init.d/functions

prog=imunify-nsqd
exec=/usr/sbin/$prog
lockfile=/var/lock/subsys/$prog
pidfile=/var/run/$prog.pid

close_fds() {
    for I in {0..65535}; do
        eval "exec ${I}<&-"
    done
}

start() {
    [ -x $exec ] || exit 5
    umask 077

    if status -p $pidfile -b $exec $prog >/dev/null; then
        echo "$prog is already running"
        return 0
    fi

    echo -n $"Starting $prog: "
    (
        cd /
        close_fds
        export GOGC=10
        setsid $exec --data-path /var/lib/imunify-nsq --use-unix-sockets --tcp-address /var/run/imunify-nsqd.sock --http-address /var/run/imunify-nsqd-http.sock &
    )
    success "$prog startup"
    echo
    touch $lockfile
    return 0
}

stop() {
    echo -n $"Shutting down $prog: "
    killproc -p "$pidfile" $exec
    retval=$?
    echo
    [ $retval -eq 0 ] && rm -f $lockfile
    return $retval
}

restart() {
    stop
    start
}

case "$1" in
    start|stop|restart)
        $1
        ;;
    force-reload)
        restart
        ;;
    status)
        status -p $pidfile -b $exec $prog
        ;;
    try-restart|condrestart)
        if status -p $pidfile -b $exec $prog >/dev/null ; then
            restart
        fi
        ;;
    reload)
        # If config can be reloaded without restarting, implement it here,
        # remove the "exit", and add "reload" to the usage message below.
        # For example:
        # status $prog >/dev/null || exit 7
        # killproc $prog -HUP
        action $"Service ${0##*/} does not support the reload action: " /bin/false
        exit 3
        ;;
    *)
        echo $"Usage: $0 {start|stop|status|restart|try-restart|force-reload}"
        exit 2
esac
