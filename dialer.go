package redisutils

import (
	"net"
	"time"

	"github.com/garyburd/redigo/redis"
)

type Dialer func() (redis.Conn, error)

func netDial(network, addr string) (net.Conn, error) {
	c, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	tc := c.(*net.TCPConn)
	if err := tc.SetKeepAlive(true); err != nil {
		return nil, err
	}
	if err := tc.SetKeepAlivePeriod(1 * time.Minute); err != nil {
		return nil, err
	}
	return tc, nil
}

func Dial(addr string) (redis.Conn, error) {
	c, err := redis.Dial("tcp", addr, redis.DialNetDial(netDial))
	if err != nil {
		return nil, err
	}
	return c, err
}

func NewDialer(addr string) Dialer {
	return func() (redis.Conn, error) {
		return Dial(addr)
	}
}
