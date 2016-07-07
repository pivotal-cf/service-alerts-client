package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
		notificationServer              *ghttp.Server
		uaaServer                       *ghttp.Server
		cfServer                        *ghttp.Server
		cfAuthRequestHandler            http.HandlerFunc
		notificationsAuthRequestHandler http.HandlerFunc
		runningBin                      *gexec.Session
		stdout                          bytes.Buffer
		stderr                          bytes.Buffer
		configFilePath                  string
		spaceGUIDFromCF                 = "3e6ca4d8-738f-46cb-989b-14290b887b47"
		cfApiUsername                   = "some-cf-user"
		cfApiPassword                   = "some-cf-password"
		cfOrgName                       = "test-org"
		cfSpaceName                     = "some-cf-space"
		product                         = "some-product"
		subject                         = "some-subject"
		serviceInstanceID               string
		replyTo                         string
		content                         = "some content"
		uaaClientID                     = "some-notifications-client-id"
		uaaClientSecret                 = "some-notifications-client-secret"
		cfToken                         = "cf-token"
		notificationsToken              = "notifications-token"
		requestMap                      map[string]string
	)

	BeforeEach(func() {
		notificationServer = ghttp.NewServer()
		uaaServer = ghttp.NewServer()
		cfServer = ghttp.NewServer()
		replyTo = "foo@bar.com"
		serviceInstanceID = "some-service-instance"

		cfAuthRequestHandler = ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/oauth/token", ""),
			ghttp.VerifyBasicAuth("cf", ""),
			ghttp.VerifyFormKV("grant_type", "password"),
			ghttp.VerifyFormKV("username", cfApiUsername),
			ghttp.VerifyFormKV("password", cfApiPassword),
			ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
				"access_token": cfToken,
				"token_type":   "bearer",
				"expires_in":   43199,
				"scope":        "cloud_controller.read",
				"jti":          "a-id-for-cf-token",
			}, http.Header{}),
		)

		notificationsAuthRequestHandler = ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/oauth/token", ""),
			ghttp.VerifyBasicAuth(uaaClientID, uaaClientSecret),
			ghttp.VerifyFormKV("grant_type", "client_credentials"),
			ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
				"access_token": notificationsToken,
				"token_type":   "bearer",
				"expires_in":   43199,
				"scope":        "clients.read password.write clients.secret clients.write uaa.admin scim.write scim.read",
				"jti":          "a-id-for-notifications-token",
			}, http.Header{}),
		)
	})

	AfterEach(func() {
		if notificationServer.HTTPTestServer != nil {
			notificationServer.Close()
		}
		if uaaServer.HTTPTestServer != nil {
			uaaServer.Close()
		}
		if cfServer.HTTPTestServer != nil {
			cfServer.Close()
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

		cfApiURL := ""
		if cfServer.HTTPTestServer != nil {
			cfApiURL = cfServer.URL()
		}

		configFile, err := ioutil.TempFile("", "service-alerts-integration-tests")
		Expect(err).NotTo(HaveOccurred())
		defer configFile.Close()
		configFilePath = configFile.Name()
		config := client.Config{
			CloudController: client.CloudController{
				URL:      cfApiURL,
				User:     cfApiUsername,
				Password: cfApiPassword,
			},
			NotificationTarget: client.NotificationTarget{
				URL:     notificationServerURL,
				CFOrg:   cfOrgName,
				CFSpace: cfSpaceName,
				ReplyTo: replyTo,
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

		stdout = bytes.Buffer{}
		stderr = bytes.Buffer{}
		cmd := exec.Command(
			sendServiceAlertsBin,
			"-config", configFilePath,
			"-product", product,
			"-service-instance", serviceInstanceID,
			"-subject", subject,
			"-content", content,
		)
		runningBin, err = gexec.Start(cmd, io.MultiWriter(GinkgoWriter, &stdout), io.MultiWriter(GinkgoWriter, &stderr))
		Expect(err).NotTo(HaveOccurred())
		runningBin = runningBin.Wait(time.Second * 3)
	})

	captureActualRequest := func(_ http.ResponseWriter, req *http.Request) {
		var err error
		actualRequest, err := ioutil.ReadAll(req.Body)
		req.Body.Close()
		Expect(err).ShouldNot(HaveOccurred())
		requestMap = map[string]string{}
		Expect(json.Unmarshal(actualRequest, &requestMap)).To(Succeed())
	}

	orgQueryHandler := func(fixturePath string) http.HandlerFunc {
		body, err := ioutil.ReadFile(fixturePath)
		Expect(err).NotTo(HaveOccurred())

		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/v2/organizations", fmt.Sprintf("q=name:%s", cfOrgName)),
			ghttp.VerifyHeader(http.Header{
				"Authorization": {fmt.Sprintf("Bearer %s", cfToken)},
			}),
			ghttp.RespondWith(http.StatusOK, body, http.Header{}),
			captureActualRequest,
		)
	}

	spaceQueryHandler := func(fixturePath string) http.HandlerFunc {
		body, err := ioutil.ReadFile(fixturePath)
		Expect(err).NotTo(HaveOccurred())

		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/v2/organizations/97160533-c474-41dc-8068-4354171361d9/spaces", fmt.Sprintf("q=name:%s", cfSpaceName)),
			ghttp.VerifyHeader(http.Header{
				"Authorization": {fmt.Sprintf("Bearer %s", cfToken)},
			}),
			ghttp.RespondWith(http.StatusOK, body, http.Header{}),
			captureActualRequest,
		)
	}

	Context("when the UAA server returns a success response and a token for the CF API user", func() {
		BeforeEach(func() {
			uaaServer.AppendHandlers(cfAuthRequestHandler, notificationsAuthRequestHandler)

			notificationServer.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
				ghttp.VerifyHeader(http.Header{
					"X-NOTIFICATIONS-VERSION": {"1"},
					"Authorization":           {fmt.Sprintf("Bearer %s", notificationsToken)},
				}),
				captureActualRequest,
			))
		})

		Context("when the CF API returns two success responses", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					orgQueryHandler("fixtures/cf_orgs_response.json"),
					spaceQueryHandler("fixtures/cf_org_spaces_response.json"),
				)
			})

			It("exits with 0", func() {
				Expect(runningBin.ExitCode()).To(Equal(0))
			})

			It("obtains two tokens from UAA", func() {
				Expect(uaaServer.ReceivedRequests()).To(HaveLen(2))
			})

			It("calls the CF API to list orgs and spaces", func() {
				Expect(cfServer.ReceivedRequests()).To(HaveLen(2))
			})
		})
	})

	Context("when UAA server returns a success response and a token for the Notifications client", func() {
		BeforeEach(func() {
			uaaServer.AppendHandlers(cfAuthRequestHandler, notificationsAuthRequestHandler)
			cfServer.AppendHandlers(
				orgQueryHandler("fixtures/cf_orgs_response.json"),
				spaceQueryHandler("fixtures/cf_org_spaces_response.json"),
			)
		})

		Context("when the notification service returns a success response", func() {
			BeforeEach(func() {
				notificationServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
						ghttp.VerifyHeader(http.Header{
							"X-NOTIFICATIONS-VERSION": {"1"},
							"Authorization":           {fmt.Sprintf("Bearer %s", notificationsToken)},
						}),
						captureActualRequest,
					),
				)
			})

			It("exits with 0", func() {
				Expect(runningBin.ExitCode()).To(Equal(0))
			})

			It("obtains two tokens from UAA", func() {
				Expect(uaaServer.ReceivedRequests()).To(HaveLen(2))
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
						ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
						ghttp.VerifyHeader(http.Header{
							"X-NOTIFICATIONS-VERSION": {"1"},
							"Authorization":           {fmt.Sprintf("Bearer %s", notificationsToken)},
						}),
						captureActualRequest,
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
						ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
						ghttp.VerifyHeader(http.Header{
							"X-NOTIFICATIONS-VERSION": {"1"},
							"Authorization":           {fmt.Sprintf("Bearer %s", notificationsToken)},
						}),
						captureActualRequest,
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
							ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
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
					ghttp.RespondWith(http.StatusInternalServerError, "{}", http.Header{}),
				))
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})

			It("logs the error", func() {
				Expect(stderr.String()).To(ContainSubstring("UAA expected to return HTTP 200, got 500."))
			})
		})

		Context("UAA server returns an unparseable response", func() {
			BeforeEach(func() {
				uaaServer.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/oauth/token", ""),
					ghttp.RespondWith(http.StatusOK, "oi", http.Header{}),
				))
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})

			It("logs the error", func() {
				Expect(stderr.String()).To(ContainSubstring("UAA response not parseable:"))
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

		Context("UAA server returns unauthorized", func() {
			Context("the CF API user is unauthorized", func() {
				BeforeEach(func() {
					uaaServer.AppendHandlers(ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/oauth/token", ""),
						ghttp.RespondWith(http.StatusUnauthorized, `{"error":"unauthorized","error_description":"Bad credentials"}`, http.Header{}),
					))
				})

				It("exits with non-zero", func() {
					Expect(runningBin.ExitCode()).NotTo(Equal(0))
				})

				It("logs the error", func() {
					Expect(stderr.String()).To(ContainSubstring("UAA expected to return HTTP 200, got 401."))
				})
			})

			Context("the notifications client is unauthorized", func() {
				BeforeEach(func() {
					uaaServer.AppendHandlers(
						cfAuthRequestHandler,
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/oauth/token", ""),
							ghttp.RespondWith(http.StatusUnauthorized, `{"error":"unauthorized","error_description":"Bad credentials"}`, http.Header{}),
						),
					)

					cfServer.AppendHandlers(
						orgQueryHandler("fixtures/cf_orgs_response.json"),
						spaceQueryHandler("fixtures/cf_org_spaces_response.json"),
					)
				})

				It("exits with non-zero", func() {
					Expect(runningBin.ExitCode()).NotTo(Equal(0))
				})

				It("logs the error", func() {
					Expect(stderr.String()).To(ContainSubstring("UAA expected to return HTTP 200, got 401."))
				})
			})
		})
	})

	Describe("CF API failures", func() {
		BeforeEach(func() {
			uaaServer.AppendHandlers(cfAuthRequestHandler)
		})

		Context("CF is unreachable", func() {
			BeforeEach(func() {
				cfServer.Close()
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})
		})

		Context("the organization does not exist", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(orgQueryHandler("fixtures/cf_no_matches_response.json"))
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})

			It("logs the error", func() {
				Expect(stderr.String()).To(ContainSubstring(fmt.Sprintf("CF org not found: '%s'", cfOrgName)))
			})
		})

		Context("the organization response is unparseable", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(orgQueryHandler("fixtures/cf_unparseable_response.json"))
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})

			It("logs an error", func() {
				Expect(stderr.String()).To(ContainSubstring("CF response not parseable:"))
			})
		})

		Context("the space does not exist", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					orgQueryHandler("fixtures/cf_orgs_response.json"),
					spaceQueryHandler("fixtures/cf_no_matches_response.json"),
				)
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})

			It("logs the error", func() {
				Expect(stderr.String()).To(ContainSubstring(fmt.Sprintf("CF space not found: '%s'", cfSpaceName)))
			})
		})

		Context("the space response is unparseable", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					orgQueryHandler("fixtures/cf_orgs_response.json"),
					spaceQueryHandler("fixtures/cf_unparseable_response.json"),
				)
			})

			It("exits with non-zero", func() {
				Expect(runningBin.ExitCode()).NotTo(Equal(0))
			})

			It("logs an error", func() {
				Expect(stderr.String()).To(ContainSubstring("CF response not parseable:"))
			})
		})
	})
})
