package integration_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

// send-service-alert
// -config config.yml
// -product "MySQL"
// -service-instance "instance" #optional
// -subject "Alert for MySQL service"
// -content "Failed backup"`
//
// config.yml
//
// notification_target:
// url: https://notifcations.cf.com
// cf_space_guid: some-guid
// authentication:
//   uaa:
//     url: https://10.10.10.10:5493
//     client_id: least-privileged-client
//     client_secret: password
var _ = Describe("send-service-alert executable", func() {
	var (
		notificationServer *ghttp.Server
		runningBin         *gexec.Session

		spaceGuid         = "some-space"
		product           = "some-product"
		subject           = "some-subject"
		serviceInstanceID = "some-service-instance"
		replyTo           = "foo@bar.com"
		content           = "some content"
	)

	BeforeEach(func() {
		text := fmt.Sprintf("Alert from %s, service instance %s:\n\n%s", product, serviceInstanceID, content)
		sendNotificationReqBody := fmt.Sprintf(`{
			"kind_id": "UPSERT_ME_TO_NOTIFICATION_SERVICE",
			"subject": "[Service Alert][%s] %s",
			"text": "%s",
			"reply_to": "%s",
			}`, product, subject, text, replyTo)

		notificationServer = ghttp.NewServer()
		notificationServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGuid)),
				ghttp.VerifyHeader(http.Header{
					"X-NOTIFICATIONS-VERSION": {"1"},
					"Authorization":           {"Bearer GET_ME_FROM_UAA"},
				}),
				ghttp.VerifyJSON(sendNotificationReqBody),
			),
		)
	})

	AfterEach(func() {
		notificationServer.Close()
	})

	JustBeforeEach(func() {
		cmd := exec.Command(sendServiceAlertsBin)
		var err error
		runningBin, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		runningBin = runningBin.Wait(time.Second * 3)
	})

	It("exits with 0", func() {
		Expect(runningBin.ExitCode()).To(Equal(0))
	})

	// curl -i -X POST \
	//   -H "X-NOTIFICATIONS-VERSION: 1" \
	//   -H "Authorization: Bearer $TOKEN" \
	//   -d '{"kind_id":"$KIND", "subject":"$PARAMETER", "text":"$PARAMETER", "reply_to": "$FROM_CONFIG"}' \
	//   http://notifications.example.com/spaces/space-guid
	It("calls the notification service", func() {
		Expect(notificationServer.ReceivedRequests()).To(HaveLen(1))
	})
})
