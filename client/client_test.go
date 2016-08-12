package client

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	Describe("retryTimeLimit()", func() {
		Context("when http retry time limit is not configured", func() {
			It("returns the default time limit", func() {
				timeLimit := retryTimeLimit(Config{})
				Expect(timeLimit).To(Equal(time.Second * 60))
			})
		})

		Context("when http retry time limit is greater than zero", func() {
			It("returns the configured time limit", func() {
				timeLimit := retryTimeLimit(Config{HTTPRetryTimeLimitSeconds: 10})
				Expect(timeLimit).To(Equal(time.Second * 10))
			})
		})

		Context("when http retry time limit is zero", func() {
			It("returns the default time limit", func() {
				timeLimit := retryTimeLimit(Config{HTTPRetryTimeLimitSeconds: 0})
				Expect(timeLimit).To(Equal(time.Second * 60))
			})
		})
	})
})
