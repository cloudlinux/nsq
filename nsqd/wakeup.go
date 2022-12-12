package nsqd

import (
	"io/fs"
	"net"
	"os"
	"path"
	"sync"
)

const socketDir = "/run"

type WakeUp interface {
	NewMessageInTopic(topic *Topic)
	Connected(channelName string)
	Disconnected(channelName string)
	Loop()
}

type wakeup struct {
	sync.RWMutex

	channels       sync.Map
	newMessageChan chan *Topic
	nsqd           *NSQD
}

var _ WakeUp = &wakeup{}

func newWakeup(nsqd *NSQD) WakeUp {
	return &wakeup{
		newMessageChan: make(chan *Topic),
		nsqd:           nsqd,
	}
}

func (w *wakeup) NewMessageInTopic(topic *Topic) {
	w.nsqd.logf(LOG_DEBUG, "new message in topic: %s", topic.name)
	w.newMessageChan <- topic
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
	var topic *Topic
	w.nsqd.logf(LOG_DEBUG, "wakeup loop is running...")
	for {
		select {
		case <-w.nsqd.exitChan:
			goto exit
		case topic = <-w.newMessageChan:
			w.nsqd.logf(LOG_DEBUG, "new message in topic received: %s", topic.name)

			topic.RLock()
			var channels []*Channel
			for _, channel := range topic.channelMap {
				channels = append(channels, channel)
			}
			topic.RUnlock()

			for _, channel := range channels {
				if !isSocket(channel.name) {
					continue
				}

				value, ok := w.channels.Load(channel.name)
				if ok {
					if value == stateSubscribed {
						w.nsqd.logf(LOG_DEBUG, "consumer already connected: %s", channel.name)
						continue
					} else if value == stateInit {
						w.nsqd.logf(LOG_DEBUG, "consumer already launched: %s", channel.name)
						continue
					}
				}

				w.nsqd.logf(LOG_DEBUG, "wakeup client: %s", channel.name)
				err := w.up(channel.name)
				if err != nil {
					w.nsqd.logf(LOG_DEBUG, "failed to connect to %s: %s", channel.name, err)
					continue
				}
			}
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
	w.Lock()
	defer w.Unlock()
	err := openConnect(socketPath)
	if err != nil {
		return err
	}
	// TODO: unset stateInit after timeout when client won't connect
	w.channels.Store(channelName, stateInit)
	return nil
}

func openConnect(addr string) error {
	conn, err := net.Dial("unix", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}
