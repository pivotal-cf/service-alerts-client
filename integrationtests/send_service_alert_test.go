package integration_test

import (
	"encoding/json"
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
		uaaServer          *ghttp.Server
		runningBin         *gexec.Session
		configFilePath     string
		spaceGuid          = "some-space"
		product            = "some-product"
		subject            = "some-subject"
		serviceInstanceID  string
		replyTo            string
		content            = "some content"
		uaaClientID        = "some-client-id"
		uaaClientSecret    = "some-client-secret"
		token              = "a-token"
	)

	BeforeEach(func() {
		notificationServer = ghttp.NewServer()
		uaaServer = ghttp.NewServer()
		replyTo = "foo@bar.com"
		serviceInstanceID = "some-service-instance"
	})

	AfterEach(func() {
		if notificationServer.HTTPTestServer != nil {
			notificationServer.Close()
		}
		if uaaServer.HTTPTestServer != nil {
			uaaServer.Close()
		}
		Expect(os.Remove(configFilePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		notificationServerURL := ""
		if notificationServer.HTTPTestServer != nil {
			notificationServerURL = notificationServer.URL()
		}

		uaaURL := ""
		if uaaServer.HTTPTestServer != nil {
			uaaURL = uaaServer.URL()
		}

		configFile, err := ioutil.TempFile("", "service-alerts-integration-tests")
		Expect(err).NotTo(HaveOccurred())
		defer configFile.Close()
		configFilePath = configFile.Name()
		config := client.Config{
			NotificationTarget: client.NotificationTarget{
				URL:         notificationServerURL,
				CFSpaceGUID: spaceGuid,
				ReplyTo:     replyTo,
				Authentication: client.Authentication{
					UAA: client.UAA{
						URL:          uaaURL,
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

		cmd := exec.Command(
			sendServiceAlertsBin,
			"-config", configFilePath,
			"-product", product,
			"-service-instance", serviceInstanceID,
			"-subject", subject,
			"-content", content,
		)
		runningBin, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		runningBin = runningBin.Wait(time.Second * 3)
	})

	Context("when UAA server returns a success resonse and a token", func() {
		var requestMap map[string]string

		caputureActualRequest := func(_ http.ResponseWriter, req *http.Request) {
			var err error
			actualRequest, err := ioutil.ReadAll(req.Body)
			req.Body.Close()
			Expect(err).ShouldNot(HaveOccurred())
			requestMap = map[string]string{}
			Expect(json.Unmarshal(actualRequest, &requestMap)).To(Succeed())
		}

		BeforeEach(func() {
			uaaServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/oauth/token", ""),
				ghttp.VerifyBasicAuth(uaaClientID, uaaClientSecret),
				ghttp.VerifyFormKV("grant_type", "client_credentials"),
				ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
					"access_token": token,
					"token_type":   "bearer",
					"expires_in":   43199,
					"scope":        "clients.read password.write clients.secret clients.write uaa.admin scim.write scim.read",
					"jti":          "a-token",
				}, http.Header{}),
			))
		})

		Context("when the notification service returns a success response", func() {
			BeforeEach(func() {
				notificationServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGuid)),
						ghttp.VerifyHeader(http.Header{
							"X-NOTIFICATIONS-VERSION": {"1"},
							"Authorization":           {fmt.Sprintf("Bearer %s", token)},
						}),
						caputureActualRequest,
					),
				)
			})

			It("exits with 0", func() {
				Expect(runningBin.ExitCode()).To(Equal(0))
			})

			It("obtains a token from UAA", func() {
				Expect(uaaServer.ReceivedRequests()).To(HaveLen(1))
			})

			It("calls the notification service", func() {
				Expect(notificationServer.ReceivedRequests()).To(HaveLen(1))

				Expect(requestMap).To(HaveKeyWithValue("kind_id", client.DummyKindID))
				Expect(requestMap).To(HaveKeyWithValue("subject", "[Service Alert]["+product+"] "+subject))
				Expect(requestMap).To(HaveKeyWithValue("text", ContainSubstring(fmt.Sprintf("Alert from %s, service instance %s:", product, serviceInstanceID))))
				Expect(requestMap).To(HaveKeyWithValue("text", ContainSubstring(content)))
				Expect(requestMap).To(HaveKeyWithValue("text", ContainSubstring("[Alert generated at ")))
				Expect(requestMap).To(HaveKeyWithValue("reply_to", replyTo))
			})
		})

		Context("when reply-to is not configured", func() {
			BeforeEach(func() {
				replyTo = ""

				notificationServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGuid)),
						ghttp.VerifyHeader(http.Header{
							"X-NOTIFICATIONS-VERSION": {"1"},
							"Authorization":           {fmt.Sprintf("Bearer %s", token)},
						}),
						caputureActualRequest,
					),
				)
			})

			It("exits with 0", func() {
				Expect(runningBin.ExitCode()).To(Equal(0))
			})

			It("calls the notification service", func() {
				Expect(notificationServer.ReceivedRequests()).To(HaveLen(1))

				Expect(requestMap).To(HaveKeyWithValue("kind_id", client.DummyKindID))
				Expect(requestMap).To(HaveKeyWithValue("subject", "[Service Alert]["+product+"] "+subject))
				Expect(requestMap).To(HaveKeyWithValue("text", ContainSubstring(fmt.Sprintf("Alert from %s, service instance %s:", product, serviceInstanceID))))
				Expect(requestMap).To(HaveKeyWithValue("text", ContainSubstring(content)))
				Expect(requestMap).To(HaveKeyWithValue("text", ContainSubstring("[Alert generated at ")))
				Expect(requestMap).NotTo(HaveKey("reply_to"))
			})
		})

		Context("when service instance id is not configured", func() {
			BeforeEach(func() {
				serviceInstanceID = ""

				notificationServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGuid)),
						ghttp.VerifyHeader(http.Header{
							"X-NOTIFICATIONS-VERSION": {"1"},
							"Authorization":           {fmt.Sprintf("Bearer %s", token)},
						}),
						caputureActualRequest,
					),
				)
			})

			It("exits with 0", func() {
				Expect(runningBin.ExitCode()).To(Equal(0))
			})

			It("calls the notification service", func() {
				Expect(notificationServer.ReceivedRequests()).To(HaveLen(1))

				Expect(requestMap).To(HaveKeyWithValue("kind_id", client.DummyKindID))
				Expect(requestMap).To(HaveKeyWithValue("subject", "[Service Alert]["+product+"] "+subject))
				Expect(requestMap).To(HaveKeyWithValue("text", ContainSubstring(fmt.Sprintf("Alert from %s:", product))))
				Expect(requestMap).To(HaveKeyWithValue("text", ContainSubstring(content)))
				Expect(requestMap).To(HaveKeyWithValue("text", ContainSubstring("[Alert generated at ")))
				Expect(requestMap).To(HaveKeyWithValue("reply_to", replyTo))
			})
		})

		Describe("notifications service failures", func() {
			Context("when the notifications service returns HTTP 500", func() {
				BeforeEach(func() {
					notificationServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGuid)),
							ghttp.RespondWith(http.StatusInternalServerError, "something went wrong", http.Header{}),
						),
					)
				})

				It("exits with non-zero", func() {
					Expect(runningBin.ExitCode()).NotTo(Equal(0))
				})
			})

			Context("notifications server can't be reached", func() {
				BeforeEach(func() {
					notificationServer.Close()
				})

				It("exits with non-zero", func() {
					Expect(runningBin.ExitCode()).NotTo(Equal(0))
				})
			})
		})
	})

	Describe("UAA failures", func() {
		Context("uaa server returns 500", func() {
			BeforeEach(func() {
				uaaServer.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/oauth/token", ""),
					ghttp.VerifyBasicAuth(uaaClientID, uaaClientSecret),
					ghttp.VerifyFormKV("grant_type", "client_credentials"),
					ghttp.RespondWith(http.StatusInternalServerError, "{}", http.Header{}),
				))
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})
		})

		Context("UAA server returns an unparseable response", func() {
			BeforeEach(func() {
				uaaServer.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/oauth/token", ""),
					ghttp.VerifyBasicAuth(uaaClientID, uaaClientSecret),
					ghttp.VerifyFormKV("grant_type", "client_credentials"),
					ghttp.RespondWith(http.StatusInternalServerError, "oi", http.Header{}),
				))
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})
		})

		Context("UAA server returns unauthorized", func() {
			BeforeEach(func() {
				uaaServer.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/oauth/token", ""),
					ghttp.VerifyBasicAuth(uaaClientID, uaaClientSecret),
					ghttp.VerifyFormKV("grant_type", "client_credentials"),
					ghttp.RespondWith(http.StatusUnauthorized, `{"error":"unauthorized","error_description":"Bad credentials"}`, http.Header{}),
				))
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})
		})

		Context("UAA server can't be reached", func() {
			BeforeEach(func() {
				uaaServer.Close()
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})
		})
	})
})
