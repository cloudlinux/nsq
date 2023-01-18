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

# Source function library.
. /etc/init.d/functions

RETVAL=0
PIDFILE=/var/run/imunify-nsqd.pid

prog=imunify-nsqd
exec=/usr/sbin/$prog
lockfile=/var/lock/subsys/$prog

start() {
        [ -x $exec ] || exit 5
        echo -n "Starting $prog: "
        setsid $exec --pidfile $PIDFILE --data-path /var/lib/imunify-nsqd --max-bytes-per-queue 104857600 --max-bytes-per-file 10485760 --use-unix-sockets --tcp-address /var/run/imunify-nsqd.sock --http-address /var/run/imunify-nsqd-http.sock &
        RETVAL=$?
        if [ $RETVAL -eq 0 ]
        then
            touch $lockfile
        fi
        echo
        return $RETVAL
}

stop() {
        echo -n "Shutting down $prog: "
        # "-d 60" equivalent to TimeoutStopSec=60 for systemd
        killproc -p "$PIDFILE" -d 60 $exec
        RETVAL=$?
        echo
        [ $RETVAL -eq 0 ] && rm -f $lockfile
        return $RETVAL
}

show_status() {
        boot_time="$(stat -c %Y /proc)"
        if [ -f $lockfile ]; then
          status="Daemonized"
          start_time="$(stat -c %Y $lockfile)"
        else
          pid=$(/sbin/pidof -c -m -o $$ -o $PPID -o %PPID -x "$exec")
          if [[ "$pid" -eq "" ]]; then
             status="Stopped"
             start_time=$boot_time
          else
             status="Applying database migrations"
             start_time="$(stat -c %Y /proc/$pid)"
          fi
        fi
        echo "StatusText=$status"
        echo "ExecMainStartTimestampMonotonic=$((start_time-boot_time))000000"
        return 0
}

rhstatus() {
        status -p "$PIDFILE" -l $prog $exec
        RETVAL=$?
        return $RETVAL
}

restart() {
        stop
        start
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
  reload)
        exit 3
        ;;
  force-reload)
        restart
        ;;
  status)
        rhstatus
        ;;
  show)
        show_status
        ;;
  condrestart|try-restart)
        rhstatus >/dev/null 2>&1 || exit 0
        restart
        ;;
  *)
        echo $"Usage: $0 {start|stop|restart|condrestart|try-restart|reload|force-reload|status}"
        exit 3
esac

exit $?
