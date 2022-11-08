package redisutils

import (
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/sirupsen/logrus"
)

type MonitorState string

const (
	MonitorStateConnecting   = MonitorState("connecting")
	MonitorStateConnected    = MonitorState("connected")
	MonitorStateDisconnected = MonitorState("disconnected")
)

type MonitorOptions struct {
	// wait time between two connects
	WaitDuration time.Duration
	// On state change callback
	OnStateChange func(state MonitorState)
	// Logger
	LoggerEntry *logrus.Entry
}

type MonitorOption func(*MonitorOptions)

func MonitorWaitDuration(waitDuration time.Duration) MonitorOption {
	return func(options *MonitorOptions) {
		options.WaitDuration = waitDuration
	}
}

func MonitorOnStateChange(onStateChange func(state MonitorState)) MonitorOption {
	return func(options *MonitorOptions) {
		options.OnStateChange = onStateChange
	}
}

func MonitorLogger(logger *logrus.Logger) MonitorOption {
	return func(options *MonitorOptions) {
		options.LoggerEntry = logger.WithFields(nil)
	}
}

func MonitorLoggerEntry(entry *logrus.Entry) MonitorOption {
	return func(options *MonitorOptions) {
		options.LoggerEntry = entry
	}
}

type Monitor struct {
	dial          Dialer
	channelName   string
	onMessage     func(data []byte)
	waitDuration  time.Duration
	onStateChange func(state MonitorState)
	logger        *logrus.Entry

	started          bool
	currentConn      redis.Conn
	currentConnMutex sync.Mutex
	currentState     MonitorState
}

func NewMonitor(dial Dialer, channelName string, onMessage func(data []byte), opts ...MonitorOption) *Monitor {
	options := &MonitorOptions{
		WaitDuration:  1 * time.Second,
		OnStateChange: nil,
		LoggerEntry:   logrus.StandardLogger().WithFields(nil),
	}
	for _, setter := range opts {
		setter(options)
	}

	m := &Monitor{
		dial:          dial,
		channelName:   channelName,
		onMessage:     onMessage,
		waitDuration:  options.WaitDuration,
		onStateChange: options.OnStateChange,
		logger:        options.LoggerEntry,

		started:      true,
		currentConn:  nil,
		currentState: MonitorState(""),
	}

	go m.start()

	return m
}

func (m *Monitor) Close() error {
	m.started = false

	m.currentConnMutex.Lock()
	defer m.currentConnMutex.Unlock()

	if m.currentConn != nil {
		m.currentConn.Close()
		m.currentConn = nil
	}

	return nil
}

func (m *Monitor) setState(state MonitorState) {
	if m.currentState == state {
		return
	}

	m.currentState = state

	m.logger.WithField("state", string(state)).Debug("Redis monitor state changed")

	if m.onStateChange != nil {
		m.onStateChange(state)
	}
}

func (m *Monitor) do() {
	defer func() {
		if m.currentState == MonitorStateConnected {
			m.setState(MonitorStateDisconnected)
		}
	}()

	m.setState(MonitorStateConnecting)

	conn, err := m.dial()
	if err != nil {
		m.logger.WithError(err).Warn("Redis monitor dial failed")
		return
	}

	m.currentConnMutex.Lock()
	m.currentConn = conn
	m.currentConnMutex.Unlock()

	defer func() {
		m.currentConnMutex.Lock()
		defer m.currentConnMutex.Unlock()

		if m.currentConn != nil {
			m.currentConn.Close()
		}
	}()

	psc := redis.PubSubConn{Conn: conn}
	err = psc.Subscribe(m.channelName)
	if err != nil {
		m.logger.WithError(err).Warn("Redis monitor subscribe failed")
		return
	}

	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			if m.started {
				m.onMessage(v.Data)
			}

		case redis.Subscription:
			m.setState(MonitorStateConnected)

		case error:
			if m.started {
				m.logger.WithError(v).Warn("Redis monitor receive error")
			}
			return
		}
	}
}

func (m *Monitor) start() {
	for m.started {
		m.do()

		time.Sleep(m.waitDuration)
	}
}
