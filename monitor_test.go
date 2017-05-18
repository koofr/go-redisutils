package redisutils_test

// REDIS_HOST=localhost REDIS_PORT=6379 ginkgo

import (
	"fmt"
	"os"
	"time"

	. "github.com/koofr/go-redisutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitor", func() {
	var dial Dialer

	BeforeEach(func() {
		redisHost := os.Getenv("REDIS_HOST")
		redisPort := os.Getenv("REDIS_PORT")

		if redisHost == "" || redisPort == "" {
			Skip("Missing REDIS_HOST, REDIS_PORT env variables")
		}

		addr := fmt.Sprintf("%s:%s", redisHost, redisPort)

		dial = NewDialer(addr)
	})

	It("should get message", func() {
		onMessage := func(data []byte) {
			fmt.Println("onMessage", data)
		}

		monitor := NewMonitor(dial, "redisutilstest:monitor", onMessage, 100*time.Millisecond)

		time.Sleep(200 * time.Millisecond)

		conn, err := dial()
		Expect(err).NotTo(HaveOccurred())
		err = conn.Send("PUBLISH", "redisutilstest:monitor", []byte("data"))
		Expect(err).NotTo(HaveOccurred())
		conn.Close()

		time.Sleep(5 * time.Millisecond)

		monitor.Close()
	})

})
