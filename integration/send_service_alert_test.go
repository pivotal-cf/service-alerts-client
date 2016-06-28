package integration_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/pivotal-cf/service-alerts-client/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"gopkg.in/yaml.v2"
)

var _ = Describe("send-service-alert executable", func() {
	var (
		notificationServer *ghttp.Server
		runningBin         *gexec.Session
		configFilePath     string
		spaceGuid          = "some-space"
		product            = "some-product"
		subject            = "some-subject"
		serviceInstanceID  = "some-service-instance"
		replyTo            = "foo@bar.com"
		content            = "some content"
		uaaClientID        = "some-client-id"
		uaaClientSecret    = "some-client-secret"
	)

	BeforeEach(func() {
		notificationServer = ghttp.NewServer()

		configFile, err := ioutil.TempFile("", "service-alerts-integration-tests")
		Expect(err).NotTo(HaveOccurred())
		defer configFile.Close()
		configFilePath = configFile.Name()
		config := client.Config{
			NotificationTarget: client.NotificationTarget{
				URL:         notificationServer.HTTPTestServer.URL,
				CFSpaceGUID: spaceGuid,
				ReplyTo:     replyTo,
				Authentication: client.Authentication{
					UAA: client.UAA{
						URL:          "DOES_NOT_MATTER_UNTIL_WE_GET_REAL_TOKENS",
						ClientID:     uaaClientID,
						ClientSecret: uaaClientSecret,
					},
				},
			},
		}
		configBytes, err := yaml.Marshal(config)
		Expect(err).NotTo(HaveOccurred())
		_, err = configFile.Write(configBytes)
		Expect(err).NotTo(HaveOccurred())

		// newlines must be encoded in json string literal
		text := fmt.Sprintf("Alert from %s, service instance %s:\\n\\n%s", product, serviceInstanceID, content)
		sendNotificationReqBody := fmt.Sprintf(`{
			"kind_id": "%s",
			"subject": "[Service Alert][%s] %s",
			"text": "%s",
			"reply_to": "%s"
			}`, client.DummyKindID, product, subject, text, replyTo)

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
		Expect(os.Remove(configFilePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		cmd := exec.Command(
			sendServiceAlertsBin,
			"-config", configFilePath,
			"-product", product,
			"-service-instance", serviceInstanceID,
			"-subject", subject,
			"-content", content,
		)
		var err error
		runningBin, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		runningBin = runningBin.Wait(time.Second * 3)
	})

	It("exits with 0", func() {
		Expect(runningBin.ExitCode()).To(Equal(0))
	})

	It("calls the notification service", func() {
		Expect(notificationServer.ReceivedRequests()).To(HaveLen(1))
	})
})
