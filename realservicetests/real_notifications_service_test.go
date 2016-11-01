package realservice_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pivotal-cf/service-alerts-client/client"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pborman/uuid"
	"gopkg.in/yaml.v2"
)

type Emails struct {
	Emails []Email `json:"items"`
}

type Email struct {
	Content Content `json:"Content"`
}

type Content struct {
	Headers Headers `json:"Headers"`
	Body    string  `json:"Body"`
}

type Headers struct {
	ReplyTo []string `json:"Reply-To"`
	Subject []string `json:"Subject"`
	To      []string `json:"To"`
}

var _ = Describe("sending a service alert to a real CF notifications service instance", func() {
	var (
		configFilePath  string
		mailhogURL      string
		cfOrg           string
		replyTo         = "some-reply-to-email@example.com"
		devUserEmail    = "space-developer@example.com"
		devUserPassword = "some-password-that-we-use"
		auditorUsername = "space-auditor@example.com"
		auditorPassword = "some-password-that-we-use"
		cfTimeout       = time.Second * 10
	)

	BeforeEach(func() {
		mailhogURL = envMustHave("MAILHOG_URL")

		cfAPI := envMustHave("CF_API")
		cfAdminUsername := envMustHave("CF_ADMIN_USERNAME")
		cfAdminPassword := envMustHave("CF_ADMIN_PASSWORD")
		cfOrg = "test-" + uuid.New()
		cfSpace := "test-" + uuid.New()
		Eventually(cf.Cf("api", cfAPI, "--skip-ssl-validation"), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.CfAuth(cfAdminUsername, cfAdminPassword), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("create-org", cfOrg), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("target", "-o", cfOrg), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("create-space", cfSpace), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("create-user", devUserEmail, devUserPassword), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("set-space-role", devUserEmail, cfOrg, cfSpace, "SpaceDeveloper"), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("create-user", auditorUsername, auditorPassword), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("set-space-role", auditorUsername, cfOrg, cfSpace, "SpaceAuditor"), cfTimeout).Should(gexec.Exit(0))

		configFile, err := ioutil.TempFile("", "service-alerts-integration-tests")
		Expect(err).NotTo(HaveOccurred())
		defer configFile.Close()
		configFilePath = configFile.Name()
		config := client.Config{
			CloudController: client.CloudController{
				URL:      cfAPI,
				User:     auditorUsername,
				Password: auditorPassword,
			},
			Notifications: client.Notifications{
				ServiceURL:        envMustHave("NOTIFICATIONS_SERVICE_URL"),
				SkipSSLValidation: pointerTo(true),
				CFOrg:             cfOrg,
				CFSpace:           cfSpace,
				ReplyTo:           replyTo,
				ClientID:          envMustHave("NOTIFICATIONS_CLIENT_ID"),
				ClientSecret:      envMustHave("NOTIFICATIONS_CLIENT_SECRET"),
			},
		}
		configBytes, err := yaml.Marshal(config)
		Expect(err).NotTo(HaveOccurred())
		_, err = configFile.Write(configBytes)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/messages", mailhogURL), nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		Eventually(cf.Cf("delete-org", cfOrg, "-f"), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("delete-user", devUserEmail, "-f"), cfTimeout).Should(gexec.Exit(0))
		Eventually(cf.Cf("delete-user", auditorUsername, "-f"), cfTimeout).Should(gexec.Exit(0))
		Expect(os.Remove(configFilePath)).To(Succeed())
	})

	It("sends an email", func() {
		product := "some-product"
		serviceInstanceID := "some-service-instance"
		subject := uuid.New()
		content := "some-content"

		cmd := exec.Command(
			sendServiceAlertsBin,
			"-config", configFilePath,
			"-product", product,
			"-service-instance", serviceInstanceID,
			"-subject", subject,
			"-content", content,
		)
		runningBin, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(runningBin, time.Second*3).Should(gexec.Exit(0))

		var emailContent Content
		Eventually(func() bool {
			emailsResp, err := http.Get(fmt.Sprintf("%s/api/v2/messages", mailhogURL))
			Expect(err).NotTo(HaveOccurred())
			defer emailsResp.Body.Close()
			Expect(emailsResp.StatusCode).To(Equal(http.StatusOK))
			var emails Emails
			Expect(json.NewDecoder(emailsResp.Body).Decode(&emails)).To(Succeed())
			for _, email := range emails.Emails {
				if len(email.Content.Headers.Subject) > 0 && strings.HasSuffix(email.Content.Headers.Subject[0], subject) {
					emailContent = email.Content
					return true
				}
			}

			return false
		}, time.Second*10).Should(BeTrue())

		Expect(emailContent.Headers.ReplyTo).To(ConsistOf(replyTo))
		Expect(emailContent.Headers.To).To(ConsistOf(devUserEmail))
		Expect(emailContent.Headers.Subject).To(ConsistOf(fmt.Sprintf("CF Notification: [Service Alert][%s] %s", product, subject)))
		Expect(emailContent.Body).To(ContainSubstring(fmt.Sprintf("Alert from %s", product)))
		Expect(emailContent.Body).To(ContainSubstring(fmt.Sprintf("service instance %s", serviceInstanceID)))
		Expect(emailContent.Body).To(ContainSubstring(content))
	})
})

func envMustHave(key string) string {
	value := os.Getenv(key)
	Expect(value).ToNot(BeEmpty(), fmt.Sprintf("%s must be set", key))
	return value
}

func pointerTo(b bool) *bool {
	return &b
}
