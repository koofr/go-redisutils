package redisutils_test

// REDIS_HOST=localhost REDIS_PORT=6379 ginkgo

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	. "github.com/koofr/go-redisutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitor", func() {
	var redisAddr string
	var dial Dialer

	redisChannel := "redisutilstest:monitor"

	publish := func(dial Dialer, message []byte) {
		conn, err := dial()
		Expect(err).NotTo(HaveOccurred())
		_, err = conn.Do("PUBLISH", redisChannel, message)
		Expect(err).NotTo(HaveOccurred())
		conn.Close()
	}

	BeforeEach(func() {
		redisHost := os.Getenv("REDIS_HOST")
		redisPort := os.Getenv("REDIS_PORT")

		if redisHost == "" || redisPort == "" {
			Skip("Missing REDIS_HOST, REDIS_PORT env variables")
		}

		redisAddr = fmt.Sprintf("%s:%s", redisHost, redisPort)
		dial = NewDialer(redisAddr)
	})

	It("should get message", func() {
		logger := logrus.New()
		logger.Level = logrus.DebugLevel

		var receivedMessage []byte

		onMessageChan := make(chan struct{}, 1)
		onConnectedChan := make(chan struct{}, 1)
		onDisconnectedChan := make(chan struct{}, 1)

		onMessage := func(data []byte) {
			receivedMessage = data

			onMessageChan <- struct{}{}
		}

		states := []MonitorState{}

		onStateChange := func(state MonitorState) {
			states = append(states, state)

			if state == MonitorStateConnected {
				onConnectedChan <- struct{}{}
			} else if state == MonitorStateDisconnected {
				onDisconnectedChan <- struct{}{}
			}
		}

		monitor := NewMonitor(
			dial, redisChannel, onMessage,
			MonitorWaitDuration(100*time.Millisecond),
			MonitorOnStateChange(onStateChange),
			MonitorLogger(logger),
		)

		<-onConnectedChan

		publish(dial, []byte("data"))

		<-onMessageChan

		Expect(receivedMessage).To(Equal([]byte("data")))

		monitor.Close()

		<-onDisconnectedChan

		Expect(states).To(Equal([]MonitorState{
			MonitorStateConnecting,
			MonitorStateConnected,
			MonitorStateDisconnected,
		}))
	})

	It("should reconnect", func() {
		logger := logrus.New()
		logger.Level = logrus.DebugLevel

		var receivedMessages [][]byte

		onMessageChan := make(chan struct{}, 10)
		onConnectingChan := make(chan struct{}, 1)
		onConnectedChan := make(chan struct{}, 10)
		onDisconnectedChan := make(chan struct{}, 10)

		onMessage := func(data []byte) {
			receivedMessages = append(receivedMessages, data)

			onMessageChan <- struct{}{}
		}

		states := []MonitorState{}

		onStateChange := func(state MonitorState) {
			states = append(states, state)

			if state == MonitorStateConnecting {
				onConnectingChan <- struct{}{}
			} else if state == MonitorStateConnected {
				onConnectedChan <- struct{}{}
			} else if state == MonitorStateDisconnected {
				onDisconnectedChan <- struct{}{}
			}
		}

		proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		defer proxyListener.Close()

		proxyAddr := proxyListener.Addr().String()
		proxyDial := NewDialer(proxyAddr)

		monitor := NewMonitor(
			proxyDial, redisChannel, onMessage,
			MonitorWaitDuration(50*time.Millisecond),
			MonitorOnStateChange(onStateChange),
			MonitorLogger(logger),
		)

		handleProxyConn := func() net.Conn {
			redisConn, err := net.Dial("tcp", redisAddr)
			Expect(err).NotTo(HaveOccurred())

			conn, err := proxyListener.Accept()
			Expect(err).NotTo(HaveOccurred())

			go io.Copy(redisConn, conn)
			go io.Copy(conn, redisConn)

			return conn
		}

		<-onConnectingChan
		proxyConn := handleProxyConn()
		<-onConnectedChan
		publish(dial, []byte("msg1"))
		<-onMessageChan
		proxyConn.Close()
		<-onDisconnectedChan

		<-onConnectingChan

		publish(dial, []byte("msg2"))

		proxyConn = handleProxyConn()
		<-onConnectedChan
		publish(dial, []byte("msg3"))
		<-onMessageChan
		proxyConn.Close()
		<-onDisconnectedChan

		<-onConnectingChan

		monitor.Close()

		Expect(receivedMessages).To(Equal([][]byte{
			[]byte("msg1"),
			[]byte("msg3"),
		}))

		Expect(states).To(Equal([]MonitorState{
			MonitorStateConnecting,
			MonitorStateConnected,
			MonitorStateDisconnected,
			MonitorStateConnecting,
			MonitorStateConnected,
			MonitorStateDisconnected,
			MonitorStateConnecting,
		}))
	})

	It("should be in connecting state until subscribed", func() {
		logger := logrus.New()
		logger.Level = logrus.DebugLevel

		onConnectingChan := make(chan struct{}, 1)

		states := []MonitorState{}

		onStateChange := func(state MonitorState) {
			states = append(states, state)

			if state == MonitorStateConnecting {
				onConnectingChan <- struct{}{}
			}
		}

		proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		defer proxyListener.Close()

		proxyAddr := proxyListener.Addr().String()
		proxyDial := NewDialer(proxyAddr)

		monitor := NewMonitor(
			proxyDial, redisChannel, func(data []byte) {},
			MonitorWaitDuration(50*time.Millisecond),
			MonitorOnStateChange(onStateChange),
			MonitorLogger(logger),
		)

		<-onConnectingChan
		conn, err := proxyListener.Accept()
		Expect(err).NotTo(HaveOccurred())
		go io.Copy(ioutil.Discard, conn)
		time.Sleep(100 * time.Millisecond)
		err = conn.Close()
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(100 * time.Millisecond)

		monitor.Close()

		Expect(states).To(Equal([]MonitorState{
			MonitorStateConnecting,
		}))
	})
})
