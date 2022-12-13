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

var noConnErrr = errors.New("accept unix /run/test.sock: use of closed network connection")

func TestWakeupSuccessfully(t *testing.T) {
	opts := NewOptions()
	opts.Logger = test.NewTestLogger(t)
	_, _, nsqd := mustStartNSQD(opts)
	defer os.RemoveAll(opts.DataPath)
	defer nsqd.Exit()

	topicName := "test_wakeup" + strconv.Itoa(int(time.Now().Unix()))
	topic := nsqd.GetTopic(topicName)

	testSock := "/run/test.sock"
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
		test.Equal(t, err.Error(), noConnErrr.Error())
	})

	timedout = waitTimeout(&wg, 50*time.Millisecond)
	test.Equal(t, timedout, true)
}

func TestWakeupWithoutRightClient(t *testing.T) {
	opts := NewOptions()
	opts.Logger = test.NewTestLogger(t)
	_, _, nsqd := mustStartNSQD(opts)
	defer os.RemoveAll(opts.DataPath)
	defer nsqd.Exit()

	topicName := "test_wakeup" + strconv.Itoa(int(time.Now().Unix()))
	topic := nsqd.GetTopic(topicName)

	// create channel
	_ = topic.GetChannel("different-channel-name")

	testSock := "/run/test.sock"
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
		test.Equal(t, err.Error(), noConnErrr.Error())
	})
	timedout := waitTimeout(&wg, time.Second)
	test.Equal(t, timedout, true)
}

func TestWakeupWithConnectedClient(t *testing.T) {
	opts := NewOptions()
	opts.Logger = test.NewTestLogger(t)
	opts.ClientTimeout = 60 * time.Second
	tcpAddr, _, nsqd := mustStartNSQD(opts)
	defer os.RemoveAll(opts.DataPath)
	defer nsqd.Exit()

	testSock := "/run/test.sock"
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
		test.Equal(t, err.Error(), noConnErrr.Error())
	})
	timedout := waitTimeout(&wg, 50*time.Millisecond)
	test.Equal(t, timedout, true)
}

func TestWakeupWithBrokenSocket(t *testing.T) {
	opts := NewOptions()
	opts.Logger = test.NewTestLogger(t)
	_, _, nsqd := mustStartNSQD(opts)
	defer os.RemoveAll(opts.DataPath)
	defer nsqd.Exit()

	testSock := "/run/test.sock"
	defer os.Remove(testSock)

	topicName := "test_wakeup" + strconv.Itoa(int(time.Now().Unix()))
	topic := nsqd.GetTopic(topicName)

	// create channel
	_ = topic.GetChannel(path.Base(testSock))

	msg := NewMessage(topic.GenerateID(), []byte("test body"))

	l, err := net.Listen("unix", testSock)
	test.Nil(t, err)

	listener, ok := l.(*net.UnixListener)
	test.Equal(t, ok, true)
	listener.SetUnlinkOnClose(false)
	listener.Close() // close explicitly to prevent new connections

	t.Log("sending message with broken socket")
	topic.PutMessage(msg)

	wg := util.WaitGroupWrapper{}
	wg.Wrap(func() {
		err := acceptConnection(l)
		test.NotNil(t, err)
		test.Equal(t, err.Error(), noConnErrr.Error())
	})
	timedout := waitTimeout(&wg, 50*time.Millisecond)
	test.Equal(t, timedout, false)
	err = os.Remove(testSock)
	test.Nil(t, err)

	t.Log("sending message with no luck, preventing due to statusStartError")
	l2, err := net.Listen("unix", testSock)
	test.Nil(t, err)

	wg = util.WaitGroupWrapper{}
	wg.Wrap(func() {
		err := acceptConnection(l2)
		test.NotNil(t, err)
		test.Equal(t, err.Error(), noConnErrr.Error())
	})
	topic.PutMessage(msg)
	timedout = waitTimeout(&wg, 50*time.Millisecond)
	test.Equal(t, timedout, true)
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
