package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/craigfurman/herottp"
)

type ServiceAlertsClient struct {
	config Config
}

func New(config Config) *ServiceAlertsClient {
	return &ServiceAlertsClient{config: config}
}

func (c *ServiceAlertsClient) SendServiceAlert(product, subject, serviceInstanceID, content string) error {
	httpClient := herottp.New(herottp.Config{Timeout: time.Second * 30})

	uaaTokenReq, err := http.NewRequest("POST", fmt.Sprintf("%s/oauth/token", c.config.NotificationTarget.Authentication.UAA.URL), strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return err
	}
	uaaTokenReq.SetBasicAuth(c.config.NotificationTarget.Authentication.UAA.ClientID, c.config.NotificationTarget.Authentication.UAA.ClientSecret)
	uaaTokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	uaaTokenResp, err := httpClient.Do(uaaTokenReq)
	if err != nil {
		// TODO test reponse error
		return err
	}
	defer uaaTokenResp.Body.Close()
	// TODO test for 200
	var uaaTokenRespBody UAATokenResponse
	if err := json.NewDecoder(uaaTokenResp.Body).Decode(&uaaTokenRespBody); err != nil {
		// TODO test for bad body
		return err
	}

	notificationsServiceReqBody := SpaceNotificationRequest{
		KindID:  DummyKindID,
		Subject: fmt.Sprintf("[Service Alert][%s] %s", product, subject),
		Text:    fmt.Sprintf("Alert from %s, service instance %s:\n\n%s", product, serviceInstanceID, content),
		ReplyTo: c.config.NotificationTarget.ReplyTo,
	}
	reqBytes, err := json.Marshal(notificationsServiceReqBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/spaces/%s", c.config.NotificationTarget.URL, c.config.NotificationTarget.CFSpaceGUID), bytes.NewReader(reqBytes))
	if err != nil {
		return err
	}
	req.Header.Set("X-NOTIFICATIONS-VERSION", "1")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uaaTokenRespBody.Token))
	req.Header.Set("Content-Type", "application/json")

	_, err = httpClient.Do(req)
	return err
}
