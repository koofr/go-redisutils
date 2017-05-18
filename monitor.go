package redisutils

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
)

type Monitor struct {
	dial             Dialer
	channelName      string
	onMessage        func(data []byte)
	waitDuration     time.Duration
	started          bool
	currentConn      redis.Conn
	currentConnMutex sync.Mutex
}

func NewMonitor(dial Dialer, channelName string, onMessage func(data []byte), waitDuration time.Duration) *Monitor {
	m := &Monitor{
		dial:         dial,
		channelName:  channelName,
		onMessage:    onMessage,
		waitDuration: waitDuration,
		started:      true,
		currentConn:  nil,
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

func (m *Monitor) do() {
	conn, err := m.dial()
	if err != nil {
		log.WithError(err).Warn("Redis dial failed")
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
		log.WithError(err).Warn("Redis monitor subscribe failed")
	}

	log.Debug("Redis monitor ready")

pscLoop:
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			if m.started {
				m.onMessage(v.Data)
			}

		case redis.Subscription:

		case error:
			log.WithError(v).Warn("Redis monitor receive error")

			break pscLoop
		}
	}
}

func (m *Monitor) start() {
	for m.started {
		m.do()

		time.Sleep(m.waitDuration)
	}
}
