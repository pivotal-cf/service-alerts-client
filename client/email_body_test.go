package client

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EmailBody", func() {
	date := time.Date(2009, 11, 10, 23, 0, 1, 0, time.UTC)

	It("templates out with all values", func() {
		Expect(templateEmailBody("productName", "instanceId", "lotta content", date)).To(Equal(`Alert from productName, service instance instanceId:

lotta content

[Alert generated at 2009-11-10T23:00:01Z]`))
	})

	It("templates out without service instance", func() {
		Expect(templateEmailBody("productName", "", "lotta content", date)).To(Equal(`Alert from productName:

lotta content

[Alert generated at 2009-11-10T23:00:01Z]`))
	})
})
