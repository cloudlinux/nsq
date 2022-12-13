package nsqd

import (
	"io/fs"
	"net"
	"os"
	"path"
	"sync"
	"time"
)

const (
	socketDir = "/run"

	statusInit = iota
	statusConnected
	statusDisconnected
	statusStartError

	socketConnectionTimeout   = 5 * time.Second
	consumerConnectionTimeout = 30 * time.Second
	startupTimeout            = time.Second
)

type WakeUp interface {
	NewMessageInChannel(channelName string)
	Connected(channelName string)
	Disconnected(channelName string)
	Loop()
}

type wakeup struct {
	channels       sync.Map
	newMessageChan chan string
	nsqd           *NSQD
}

type state struct {
	status    int
	timestamp time.Time
}

func newWakeup(nsqd *NSQD) WakeUp {
	return &wakeup{
		newMessageChan: make(chan string, 100),
		nsqd:           nsqd,
	}
}

func (w *wakeup) NewMessageInChannel(channelName string) {
	w.newMessageChan <- channelName
}

func (w *wakeup) Connected(channelName string) {
	w.nsqd.logf(LOG_DEBUG, "client is connected: %s", channelName)
	w.channels.Store(channelName, state{
		status:    statusConnected,
		timestamp: time.Now(),
	})
}

func (w *wakeup) Disconnected(channelName string) {
	w.nsqd.logf(LOG_DEBUG, "client is disconnected: %s", channelName)
	w.channels.Store(channelName, state{
		status:    statusDisconnected,
		timestamp: time.Now(),
	})
}

func (w *wakeup) setState(channelName string, status int) {
	w.channels.Store(channelName, state{
		status:    status,
		timestamp: time.Now(),
	})
}

func (w *wakeup) Loop() {
	var channelName string
	w.nsqd.logf(LOG_DEBUG, "wakeup loop is running...")
	for {
		select {
		case <-w.nsqd.exitChan:
			goto exit
		case channelName = <-w.newMessageChan:
			if !isSocket(channelName) {
				w.nsqd.logf(LOG_DEBUG, "channel %s has not a socket consumer", channelName)
				continue
			}
			w.nsqd.logf(LOG_DEBUG, "new message in channel received: %s", channelName)

			value, ok := w.channels.Load(channelName)
			if ok {
				s, ok := value.(state)
				if !ok {
					w.nsqd.logf(LOG_ERROR, "invalid state for channel %s", channelName)
					continue
				}
				if s.status == statusConnected {
					w.nsqd.logf(LOG_WARN, "consumer already connected: %s", channelName)
					continue
				} else if s.status == statusInit && time.Now().Sub(s.timestamp) < consumerConnectionTimeout {
					w.nsqd.logf(LOG_WARN, "consumer already launched: %s", channelName)
					continue
				} else if s.status == statusStartError && time.Now().Sub(s.timestamp) < startupTimeout {
					w.nsqd.logf(LOG_WARN, "consumer failed, waiting %s: %s", channelName, startupTimeout)
					continue
				}
			}

			w.nsqd.logf(LOG_INFO, "starting client: %s", channelName)
			err := w.up(channelName)
			if err != nil {
				w.setState(channelName, statusStartError)
				w.nsqd.logf(LOG_ERROR, "failed to connect to %s: %s", channelName, err)
				continue
			}
			w.nsqd.logf(LOG_INFO, "client is launched: %s", channelName)
		}
	}
exit:
	close(w.newMessageChan)
	w.nsqd.logf(LOG_INFO, "WAKEUP: closing")
}

// isSocket returns true if the given path is a socket.
func isSocket(channelName string) bool {
	socketPath := path.Join(socketDir, channelName)
	fileInfo, err := os.Stat(socketPath)
	if err != nil {
		return false
	}
	return fileInfo.Mode().Type() == fs.ModeSocket
}

func (w *wakeup) up(channelName string) error {
	socketPath := path.Join(socketDir, channelName)
	err := openConnect(socketPath)
	if err != nil {
		return err
	}
	w.setState(channelName, statusInit)
	return nil
}

func openConnect(addr string) error {
	conn, err := net.DialTimeout("unix", addr, socketConnectionTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}
