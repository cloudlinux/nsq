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
	socketDir         = "/run"
	connectionTimeout = 5 * time.Second
)

type WakeUp interface {
	NewMessageInChannel(channelName string)
	Connected(channelName string)
	Disconnected(channelName string)
	Loop()
}

type wakeup struct {
	sync.RWMutex

	channels       sync.Map
	newMessageChan chan string
	nsqd           *NSQD
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
	w.channels.Store(channelName, stateSubscribed)
}

func (w *wakeup) Disconnected(channelName string) {
	w.nsqd.logf(LOG_DEBUG, "client is disconnected: %s", channelName)
	w.channels.Store(channelName, stateClosing)
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
				if value == stateSubscribed {
					w.nsqd.logf(LOG_WARN, "consumer already connected: %s", channelName)
					continue
				} else if value == stateInit {
					w.nsqd.logf(LOG_WARN, "consumer already launched: %s", channelName)
					continue
				}
			}

			w.nsqd.logf(LOG_DEBUG, "starting client: %s", channelName)
			err := w.up(channelName)
			if err != nil {
				w.nsqd.logf(LOG_ERROR, "failed to connect to %s: %s", channelName, err)
				continue
			}
			w.nsqd.logf(LOG_INFO, "client is launched: %s", channelName)
		}
		time.Sleep(500 * time.Millisecond)
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
	// TODO: unset stateInit after timeout when client won't connect
	w.channels.Store(channelName, stateInit)
	return nil
}

func openConnect(addr string) error {
	conn, err := net.DialTimeout("unix", addr, connectionTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}
