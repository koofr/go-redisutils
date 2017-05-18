package redisutils_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRedisutils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Redisutils Suite")
}
