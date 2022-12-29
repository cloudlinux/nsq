package nsqd

import (
	"errors"
	"github.com/nsqio/go-nsq"
	"net"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/nsqio/nsq/internal/test"
	"github.com/nsqio/nsq/internal/util"
)

var errNoConn = errors.New("accept unix /tmp/test.sock: use of closed network connection")

func TestWakeupSuccessfully(t *testing.T) {
	opts := NewOptions()
	opts.WakeupSocketDir = "/tmp"
	opts.Logger = test.NewTestLogger(t)
	_, _, nsqd := mustStartNSQD(opts)
	defer os.RemoveAll(opts.DataPath)
	defer nsqd.Exit()

	topicName := "test_wakeup" + strconv.Itoa(int(time.Now().Unix()))
	topic := nsqd.GetTopic(topicName)

	testSock := "/tmp/test.sock"
	defer os.Remove(testSock)

	// create channel
	_ = topic.GetChannel(path.Base(testSock))

	l, err := net.Listen("unix", testSock)
	test.Nil(t, err)
	defer l.Close()

	var id MessageID
	msg := NewMessage(id, []byte("test"))
	_ = topic.PutMessage(msg)

	wg := util.WaitGroupWrapper{}
	wg.Wrap(func() {
		err = acceptConnection(l)
		test.Nil(t, err)
	})
	timedout := waitTimeout(&wg, 50*time.Millisecond)
	test.Equal(t, timedout, false)

	// second time no wakeup should happen
	_ = topic.PutMessage(msg)

	wg = util.WaitGroupWrapper{}
	wg.Wrap(func() {
		err := acceptConnection(l)
		test.Equal(t, err.Error(), errNoConn.Error())
	})

	timedout = waitTimeout(&wg, 50*time.Millisecond)
	test.Equal(t, timedout, true)
}

func TestWakeupWithoutRightClient(t *testing.T) {
	opts := NewOptions()
	opts.WakeupSocketDir = "/tmp"
	opts.Logger = test.NewTestLogger(t)
	_, _, nsqd := mustStartNSQD(opts)
	defer os.RemoveAll(opts.DataPath)
	defer nsqd.Exit()

	topicName := "test_wakeup" + strconv.Itoa(int(time.Now().Unix()))
	topic := nsqd.GetTopic(topicName)

	// create channel
	_ = topic.GetChannel("different-channel-name")

	testSock := "/tmp/test.sock"
	defer os.Remove(testSock)

	l, err := net.Listen("unix", testSock)
	test.Nil(t, err)
	defer l.Close()

	var id MessageID
	msg := NewMessage(id, []byte("test"))
	_ = topic.PutMessage(msg)

	wg := util.WaitGroupWrapper{}
	wg.Wrap(func() {
		err := acceptConnection(l)
		test.Equal(t, err.Error(), errNoConn.Error())
	})
	timedout := waitTimeout(&wg, time.Second)
	test.Equal(t, timedout, true)
}

func TestWakeupWithConnectedClient(t *testing.T) {
	opts := NewOptions()
	opts.WakeupSocketDir = "/tmp"
	opts.Logger = test.NewTestLogger(t)
	opts.ClientTimeout = 60 * time.Second
	tcpAddr, _, nsqd := mustStartNSQD(opts)
	defer os.RemoveAll(opts.DataPath)
	defer nsqd.Exit()

	testSock := "/tmp/test.sock"
	defer os.Remove(testSock)

	topicName := "test_wakeup" + strconv.Itoa(int(time.Now().Unix()))
	topic := nsqd.GetTopic(topicName)

	// create channel
	_ = topic.GetChannel(path.Base(testSock))

	msg := NewMessage(topic.GenerateID(), []byte("test body"))
	topic.PutMessage(msg)

	conn, err := mustConnectNSQD(tcpAddr)
	test.Nil(t, err)
	defer conn.Close()

	identify(t, conn, nil, frameTypeResponse)
	sub(t, conn, topicName, path.Base(testSock))

	_, err = nsq.Ready(1).WriteTo(conn)
	test.Nil(t, err)

	l, err := net.Listen("unix", testSock)
	test.Nil(t, err)
	defer l.Close()

	wg := util.WaitGroupWrapper{}
	wg.Wrap(func() {
		err := acceptConnection(l)
		test.Equal(t, err.Error(), errNoConn.Error())
	})
	timedout := waitTimeout(&wg, 50*time.Millisecond)
	test.Equal(t, timedout, true)
}

func TestWakeupWithBrokenSocket(t *testing.T) {
	opts := NewOptions()
	opts.WakeupSocketDir = "/tmp"
	opts.Logger = test.NewTestLogger(t)
	_, _, nsqd := mustStartNSQD(opts)
	defer os.RemoveAll(opts.DataPath)
	defer nsqd.Exit()

	wakeupTested := nsqd.wakeup.(*wakeup)

	testSock := "/tmp/test.sock"
	defer os.Remove(testSock)

	topicName := "test_wakeup" + strconv.Itoa(int(time.Now().Unix()))
	topic := nsqd.GetTopic(topicName)

	channelName := path.Base(testSock)
	// create channel
	_ = topic.GetChannel(channelName)

	l, err := net.Listen("unix", testSock)
	test.Nil(t, err)

	listener, ok := l.(*net.UnixListener)
	test.Equal(t, ok, true)
	listener.SetUnlinkOnClose(false)
	err = listener.Close() // close explicitly to prevent new connections
	test.Nil(t, err)

	err = topic.PutMessage(NewMessage(topic.GenerateID(), []byte("test body")))
	test.Nil(t, err)

	t.Log("sending message to broken socket")
	wg := util.WaitGroupWrapper{}
	wg.Wrap(func() {
		err := acceptConnection(listener)
		test.NotNil(t, err)
		test.Equal(t, err.Error(), errNoConn.Error())
	})
	timedout := waitTimeout(&wg, 100*time.Millisecond)
	test.Equal(t, timedout, false)

	// waiting the message to be processed
	wg.Wrap(func() {
		for {
			_, ok := wakeupTested.channels.Load(channelName)
			if ok {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	timedout = waitTimeout(&wg, 100*time.Millisecond)
	test.Equal(t, timedout, false)

	v, ok := wakeupTested.channels.Load(channelName)
	test.Equal(t, ok, true)
	test.Equal(t, v.(state).status, statusStartError)

	t.Log("sending message to ready socket before timeout")
	err = os.Remove(testSock)
	test.Nil(t, err)

	err = topic.PutMessage(NewMessage(topic.GenerateID(), []byte("test body")))
	test.Nil(t, err)

	l1, err := net.Listen("unix", testSock)
	test.Nil(t, err)
	defer l1.Close()

	wg1 := util.WaitGroupWrapper{}
	wg1.Wrap(func() {
		err := acceptConnection(l1)
		test.NotNil(t, err)
		test.Equal(t, err.Error(), errNoConn.Error())
	})
	timedout = waitTimeout(&wg1, 50*time.Millisecond)
	test.Equal(t, timedout, true)
	newValue, ok := wakeupTested.channels.Load(channelName)
	test.Equal(t, ok, true)
	test.Equal(t, v, newValue) // value should not be changed

	t.Log("sending message to ready socket after timeout")
	err = os.Remove(testSock)
	test.Nil(t, err)
	time.Sleep(startupTimeout)

	l2, err := net.Listen("unix", testSock)
	test.Nil(t, err)
	defer l2.Close()

	err = topic.PutMessage(NewMessage(topic.GenerateID(), []byte("test body")))
	test.Nil(t, err)

	wg2 := util.WaitGroupWrapper{}
	wg2.Wrap(func() {
		err := acceptConnection(l2)
		test.Nil(t, err)
	})
	timedout = waitTimeout(&wg2, 500*time.Millisecond)
	test.Equal(t, timedout, false)

	v, ok = wakeupTested.channels.Load(channelName)

	test.Equal(t, ok, true)
	test.Equal(t, v.(state).status, statusInit) // means that service is launched

}

func acceptConnection(l net.Listener) error {
	conn, err := l.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	var buff []byte
	_, err = conn.Read(buff)
	if err != nil {
		return err
	}
	if string(buff) != "" {
		// just opening connection without any data
		return errors.New("expected empty buffer")
	}
	return nil
}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *util.WaitGroupWrapper, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}
