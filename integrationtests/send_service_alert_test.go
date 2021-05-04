// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package integration_test

import (
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
	"github.com/onsi/gomega/gbytes"
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
		cfInfoRequestHandler            http.HandlerFunc
		notificationsAuthRequestHandler http.HandlerFunc
		runningBin                      *gexec.Session
		stderr                          *gbytes.Buffer
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
		notificationServerURL           string
		uaaURL                          string
		cfApiURL                        string
		globalTimeoutSeconds            int
		cmdWaitDuration                 time.Duration
		waitForRetriesDuration          = time.Second * 3
		config                          client.Config
		skipSSLValidation               = makeBool(true)
	)

	BeforeEach(func() {
		notificationServer = ghttp.NewTLSServer()
		uaaServer = ghttp.NewTLSServer()
		cfServer = ghttp.NewTLSServer()
		skipSSLValidation = makeBool(true)

		notificationServerURL = "notification server not running"
		if notificationServer.HTTPTestServer != nil {
			notificationServerURL = notificationServer.URL()
		}

		uaaURL = "uaa server not running"
		if uaaServer.HTTPTestServer != nil {
			uaaURL = uaaServer.URL()
		}

		cfApiURL = "cf server not running"
		if cfServer.HTTPTestServer != nil {
			cfApiURL = cfServer.URL()
		}

		replyTo = "foo@bar.com"
		serviceInstanceID = "some-service-instance"

		cmdWaitDuration = time.Second * 3
		globalTimeoutSeconds = 1

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

		cfInfoRequestHandler = ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/v2/info"),
			ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]interface{}{
				"token_endpoint": uaaURL,
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
		notificationServer.Close()
		uaaServer.Close()
		cfServer.Close()
		Expect(os.Remove(configFilePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		configFile, err := ioutil.TempFile("", "service-alerts-integration-tests")
		Expect(err).NotTo(HaveOccurred())
		defer configFile.Close()
		configFilePath = configFile.Name()
		config = client.Config{
			CloudController: client.CloudController{
				URL:      cfApiURL,
				User:     cfApiUsername,
				Password: cfApiPassword,
			},
			Notifications: client.Notifications{
				ServiceURL:   notificationServerURL,
				CFOrg:        cfOrgName,
				CFSpace:      cfSpaceName,
				ReplyTo:      replyTo,
				ClientID:     uaaClientID,
				ClientSecret: uaaClientSecret,
			},
			SkipSSLValidation: skipSSLValidation,
		}
		if globalTimeoutSeconds != 0 {
			config.GlobalTimeoutSeconds = globalTimeoutSeconds
		}
		configBytes, err := yaml.Marshal(config)
		Expect(err).NotTo(HaveOccurred())
		_, err = configFile.Write(configBytes)
		Expect(err).NotTo(HaveOccurred())

		stderr = gbytes.NewBuffer()
		cmd := exec.Command(
			sendServiceAlertsBin,
			"-config", configFilePath,
			"-product", product,
			"-service-instance", serviceInstanceID,
			"-subject", subject,
			"-content", content,
		)
		runningBin, err = gexec.Start(cmd, GinkgoWriter, io.MultiWriter(GinkgoWriter, stderr))
		Expect(err).NotTo(HaveOccurred())
		Eventually(runningBin, cmdWaitDuration.Seconds()).Should(gexec.Exit())
	})

	captureActualRequest := func(_ http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		requestMap = map[string]string{}
		json.NewDecoder(req.Body).Decode(&requestMap) // Ignore errors, not every request has a body. Assertions will reveal whether body content is as expected
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

	spaceQueryForbidden := func() http.HandlerFunc {
		forbiddenBody := `{"code": 10003,"description": "You are not authorized to perform the requested action","error_code": "CF-NotAuthorized"}`

		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/v2/organizations/97160533-c474-41dc-8068-4354171361d9/spaces", fmt.Sprintf("q=name:%s", cfSpaceName)),
			ghttp.VerifyHeader(http.Header{
				"Authorization": {fmt.Sprintf("Bearer %s", cfToken)},
			}),
			ghttp.RespondWith(http.StatusForbidden, forbiddenBody, http.Header{}),
		)
	}

	orgQueryFailsHandler := func() http.HandlerFunc {
		return ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", "/v2/organizations", fmt.Sprintf("q=name:%s", cfOrgName)),
			ghttp.VerifyHeader(http.Header{
				"Authorization": {fmt.Sprintf("Bearer %s", cfToken)},
			}),
			ghttp.RespondWith(http.StatusInternalServerError, "I'm not JSON", nil),
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
			cfServer.AppendHandlers(cfInfoRequestHandler)
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
					spaceQueryHandler("fixtures/cf_org_spaces_response.json"))
			})

			It("exits with 0", func() {
				Expect(runningBin.ExitCode()).To(Equal(0))
			})

			It("obtains two tokens from UAA", func() {
				Expect(uaaServer.ReceivedRequests()).To(HaveLen(2))
			})

			It("calls the CF API to list orgs and spaces", func() {
				Expect(cfServer.ReceivedRequests()).To(HaveLen(3))
			})
		})

		Context("when SkipSSLValidation set to false", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					orgQueryHandler("fixtures/cf_orgs_response.json"),
					spaceQueryHandler("fixtures/cf_org_spaces_response.json"))
			})
			Context("when explicitly set", func() {
				BeforeEach(func() {
					skipSSLValidation = makeBool(false)
				})

				It("exits with 2", func() {
					Expect(runningBin.ExitCode()).To(Equal(2))
				})
			})

			Context("Implicitly", func() {
				BeforeEach(func() {
					skipSSLValidation = nil
				})

				It("exits with 2", func() {
					Expect(runningBin.ExitCode()).To(Equal(2))
				})
			})
		})
	})

	Context("when UAA server returns a success response and a token for the Notifications client", func() {
		BeforeEach(func() {
			uaaServer.AppendHandlers(cfAuthRequestHandler, notificationsAuthRequestHandler)
			cfServer.AppendHandlers(
				cfInfoRequestHandler,
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

		Describe("CF Notifications service failures", func() {
			Context("when the notifications service returns HTTP 500", func() {
				BeforeEach(func() {
					notificationServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
							ghttp.RespondWith(http.StatusInternalServerError, "something went wrong", http.Header{}),
						),
					)
				})

				It("should retry the the request up to the default request timeout (30s)", func() {
					By("retrying the request")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 6 attempts got 0")

					By("Logging a user error message to stderr")
					Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

					By("exiting with code 2")
					Expect(runningBin.ExitCode()).To(Equal(2))
				})
			})

			Context("when the notifications service takes more than 30 seconds to respond", func() {
				BeforeEach(func() {
					notificationServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
							func(http.ResponseWriter, *http.Request) {
								time.Sleep(31 * time.Second)
							},
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
							ghttp.RespondWith(http.StatusInternalServerError, "something went wrong", http.Header{}),
						),
					)
					cmdWaitDuration = 33 * time.Second
					globalTimeoutSeconds = 31
				})

				It("times out the request after the client timeout (30s) and retries", func() {
					By("retrying the request")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 2 attempts got 0")

					By("timing out the request")
					Expect(stderr).To(gbytes.Say(`context deadline exceeded \(Client.Timeout exceeded while awaiting headers\)`))

					By("Logging a user error message to stderr")
					Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

					By("exiting with code 2")
					Expect(runningBin.ExitCode()).To(Equal(2))
				})
			})

			Context("when the request to the notifications service has not succeeded before the configured global timeout", func() {
				BeforeEach(func() {
					notificationServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
							ghttp.RespondWith(http.StatusInternalServerError, "something went wrong", http.Header{}),
						),
					)
					cmdWaitDuration = 2 * time.Second
					globalTimeoutSeconds = 1
				})

				It("should time out", func() {
					By("Logging a user error message to stderr")
					Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

					By("exiting with code 2")
					Expect(runningBin.ExitCode()).To(Equal(2))
				})
			})

			Context("when the request to the notifications service has not succeeed before the default global timeout (60s)", func() {
				BeforeEach(func() {
					notificationServer.AllowUnhandledRequests = true
					notificationServer.UnhandledRequestStatusCode = http.StatusInternalServerError

					cmdWaitDuration = 61 * time.Second
					globalTimeoutSeconds = 0
				})

				It("should time out", func() {
					By("retrying the request")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 7 attempts got 0")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 7 attempts got 1")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 7 attempts got 2")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 7 attempts got 3")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 7 attempts got 4")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 7 attempts got 5")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 7 attempts got 6")
					Expect(stderr).NotTo(gbytes.Say("Retrying in"), "expected to give up after 7 failed attempts")

					By("Logging a user error message to stderr")
					Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

					By("exiting with code 2")
					Expect(runningBin.ExitCode()).To(Equal(2))
				})
			})

			Context("when the notifications service returns HTTP 422 Unprocessable Entity", func() {
				BeforeEach(func() {
					notificationServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
							ghttp.RespondWith(http.StatusUnprocessableEntity, `{"errors":["\"kind_id\" is a required field,\"text\" or \"html\" fields must be supplied"]}`, http.Header{}),
						),
					)
				})

				It("exits with 1", func() {
					Expect(runningBin.ExitCode()).To(Equal(1))
				})

				It("does not retry the request", func() {
					Expect(stderr).NotTo(gbytes.Say("Retrying in"))
				})

				It("logs the error", func() {
					Expect(stderr).To(gbytes.Say("CF Notifications expected to return HTTP 200, got 422."))
				})
			})

			Context("notifications server can't be reached", func() {
				BeforeEach(func() {
					notificationServer.Close()
					notificationServerURL = "http://somewhere-that-does-not-exist.io"
					cmdWaitDuration = waitForRetriesDuration
				})

				It("should retry the the request up to the time limit", func() {
					By("retrying the request")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 2 attempts got 0")

					By("Logging a user error message to stderr")
					Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

					By("exiting with code 2")
					Expect(runningBin.ExitCode()).To(Equal(2))
				})
			})

			Context("notifications service returns 404 Space not found", func() {
				BeforeEach(func() {
					notificationServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
							ghttp.RespondWith(http.StatusNotFound, `{"errors":["CloudController Failure: Space \"123\" could not be found"]}`, http.Header{}),
						),
					)
				})

				It("exits with 1", func() {
					Expect(runningBin.ExitCode()).To(Equal(1))
				})

				It("does not retry the request", func() {
					Expect(stderr).NotTo(gbytes.Say("Retrying in"))
				})

				It("logs the error", func() {
					Expect(stderr).To(gbytes.Say("CF Notifications expected to return HTTP 200, got 404."))
				})
			})

			Context("route to the notifications server does not exist", func() {
				BeforeEach(func() {
					header := http.Header{}
					header.Add("X-Cf-Routererror", "unknown_route")

					notificationServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", fmt.Sprintf("/spaces/%s", spaceGUIDFromCF)),
							ghttp.RespondWith(http.StatusNotFound, "404 Not Found: Requested route ('notifications.example.com') does not exist.", header),
						),
					)
					cmdWaitDuration = waitForRetriesDuration
				})

				It("should retry the the request up to the time limit", func() {
					By("retrying the request")
					Expect(stderr).To(gbytes.Say("Retrying in"), "expected 2 attempts got 0")

					By("Logging a user error message to stderr")
					Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

					By("exiting with code 2")
					Expect(runningBin.ExitCode()).To(Equal(2))
				})
			})
		})
	})

	Describe("UAA failures", func() {
		BeforeEach(func() {
			cfServer.AppendHandlers(cfInfoRequestHandler)
		})

		Context("uaa server returns 500", func() {
			BeforeEach(func() {
				uaaServer.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/oauth/token", ""),
					ghttp.RespondWith(http.StatusInternalServerError, "{}", http.Header{}),
				))
				cmdWaitDuration = waitForRetriesDuration
			})

			It("should retry the the request up to the time limit", func() {
				By("retrying the request")
				Expect(stderr).To(gbytes.Say("Retrying in"), "expected 2 attempts got 0")

				By("Logging a user error message to stderr")
				Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

				By("exiting with code 2")
				Expect(runningBin.ExitCode()).To(Equal(2))
			})
		})

		Context("UAA server returns an unparseable response", func() {
			BeforeEach(func() {
				uaaServer.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/oauth/token", ""),
					ghttp.RespondWith(http.StatusOK, "body", http.Header{}),
				))
			})

			It("exits with 1", func() {
				Expect(runningBin.ExitCode()).To(Equal(1))
			})

			It("logs the error", func() {
				Expect(stderr).To(gbytes.Say("UAA response not parseable:"))
			})
		})

		Context("UAA server can't be reached", func() {
			BeforeEach(func() {
				uaaServer.Close()
				uaaURL = "http://somewhere-that-does-not-exist.io"
				cmdWaitDuration = waitForRetriesDuration
			})

			It("should retry the the request up to the time limit", func() {
				By("retrying the request")
				Expect(stderr).To(gbytes.Say("Retrying in"), "expected 2 attempts got 0")

				By("Logging a user error message to stderr")
				Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

				By("exiting with code 2")
				Expect(runningBin.ExitCode()).To(Equal(2))
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

				It("exits with 1", func() {
					Expect(runningBin.ExitCode()).To(Equal(1))
				})

				It("does not retry the request", func() {
					Expect(stderr).NotTo(gbytes.Say("Retrying in"))
				})

				It("logs the error", func() {
					Expect(stderr).To(gbytes.Say("UAA expected to return HTTP 200, got 401."))
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

				It("exits with 1", func() {
					Expect(runningBin.ExitCode()).To(Equal(1))
				})

				It("logs the error", func() {
					Expect(stderr).To(gbytes.Say("UAA expected to return HTTP 200, got 401."))
				})
			})
		})
	})

	Describe("CF API failures", func() {
		BeforeEach(func() {
			uaaServer.AppendHandlers(cfAuthRequestHandler)
		})

		Context("CF API can't be reached", func() {
			BeforeEach(func() {
				cfServer.Close()
				cfApiURL = "http://somewhere-that-does-not-exist.io"
				cmdWaitDuration = waitForRetriesDuration
			})

			It("should retry the the request up to the time limit", func() {
				By("retrying the request")
				Expect(stderr).To(gbytes.Say("Retrying in"), "expected 2 attempts got 0")

				By("Logging a user error message to stderr")
				Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

				By("exiting with code 2")
				Expect(runningBin.ExitCode()).To(Equal(2))
			})
		})

		Context("CF API return 403 Forbidden", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					cfInfoRequestHandler,
					orgQueryHandler("fixtures/cf_orgs_response.json"),
					spaceQueryForbidden(),
				)
			})

			It("exits with 1", func() {
				Expect(runningBin.ExitCode()).To(Equal(1))
			})

			It("does not retry the request", func() {
				Expect(stderr).NotTo(gbytes.Say("Retrying in"))
			})

			It("logs the error", func() {
				Expect(stderr).To(gbytes.Say("CF API expected to return HTTP 200, got 403."))
			})
		})

		Context("CF API returns 500", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(cfInfoRequestHandler)
				handlers := []http.HandlerFunc{}
				// allow 6 retries
				for i := 0; i < 6; i++ {
					handlers = append(handlers, orgQueryFailsHandler())
				}
				cfServer.AppendHandlers(handlers...)
				cmdWaitDuration = waitForRetriesDuration
			})

			It("should retry the the request up to the time limit", func() {
				By("retrying the request")
				Expect(stderr).To(gbytes.Say("Retrying in"), "expected 2 attempts got 0")

				By("Logging a user error message to stderr")
				Expect(stderr).To(gbytes.Say(fmt.Sprintf("failed to send notification to org: %s, space: %s", cfOrgName, cfSpaceName)))

				By("exiting with code 2")
				Expect(runningBin.ExitCode()).To(Equal(2))
			})
		})

		Context("CF info API returns an unparseable response", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v2/info"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, "this is not json at all", http.Header{}),
					),
				)
			})

			It("exits with 1", func() {
				Expect(runningBin.ExitCode()).To(Equal(1))
			})

			It("logs an error", func() {
				Expect(stderr).To(gbytes.Say("CF response not parseable:"))
			})
		})

		Context("the organization does not exist", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					cfInfoRequestHandler,
					orgQueryHandler("fixtures/cf_no_matches_response.json"),
				)
			})

			It("exits with 1", func() {
				Expect(runningBin.ExitCode()).To(Equal(1))
			})

			It("logs the error", func() {
				Expect(stderr).To(gbytes.Say(fmt.Sprintf("CF org not found: '%s'", cfOrgName)))
			})
		})

		Context("the organization response is unparseable", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					cfInfoRequestHandler,
					orgQueryHandler("fixtures/cf_unparseable_response.json"),
				)
			})

			It("exits with 1", func() {
				Expect(runningBin.ExitCode()).To(Equal(1))
			})

			It("logs an error", func() {
				Expect(stderr).To(gbytes.Say("CF response not parseable:"))
			})
		})

		Context("the space does not exist", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					cfInfoRequestHandler,
					orgQueryHandler("fixtures/cf_orgs_response.json"),
					spaceQueryHandler("fixtures/cf_no_matches_response.json"),
				)
			})

			It("exits with 1", func() {
				Expect(runningBin.ExitCode()).To(Equal(1))
			})

			It("logs the error", func() {
				Expect(stderr).To(gbytes.Say(fmt.Sprintf("CF space not found: '%s'", cfSpaceName)))
			})
		})

		Context("the space response is unparseable", func() {
			BeforeEach(func() {
				cfServer.AppendHandlers(
					cfInfoRequestHandler,
					orgQueryHandler("fixtures/cf_orgs_response.json"),
					spaceQueryHandler("fixtures/cf_unparseable_response.json"),
				)
			})

			It("exits with 1", func() {
				Expect(runningBin.ExitCode()).To(Equal(1))
			})

			It("logs an error", func() {
				Expect(stderr).To(gbytes.Say("CF response not parseable:"))
			})
		})
	})
})
