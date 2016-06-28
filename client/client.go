package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/craigfurman/herottp"
)

type ServiceAlertsClient struct {
	config     Config
	httpClient *herottp.Client
}

func New(config Config) *ServiceAlertsClient {
	httpClient := herottp.New(herottp.Config{Timeout: time.Second * 30})
	return &ServiceAlertsClient{config: config, httpClient: httpClient}
}

func (c *ServiceAlertsClient) SendServiceAlert(product, subject, serviceInstanceID, content string) error {
	token, err := c.obtainUAAToken()
	if err != nil {
		return err
	}
	notificationRequest := c.createNotification(product, subject, serviceInstanceID, content)
	return c.sendNotification(token, notificationRequest)
}

func (c *ServiceAlertsClient) sendNotification(uaaToken string, notificationRequest SpaceNotificationRequest) error {
	reqBytes, err := json.Marshal(notificationRequest)
	if err != nil {
		return err
	}

	sendNotificationRequestURL, err := joinURL(c.config.NotificationTarget.URL, fmt.Sprintf("/spaces/%s", c.config.NotificationTarget.CFSpaceGUID))
	req, err := http.NewRequest("POST", sendNotificationRequestURL, bytes.NewReader(reqBytes))
	if err != nil {
		return err
	}

	req.Header.Set("X-NOTIFICATIONS-VERSION", "1")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uaaToken))
	req.Header.Set("Content-Type", "application/json")

	sendNotificationResponse, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if sendNotificationResponse.StatusCode != http.StatusOK {
		respBody, err := ioutil.ReadAll(sendNotificationResponse.Body)
		return fmt.Errorf("UAA expected to return HTTP 200, got %d. Body: %s%s\n", sendNotificationResponse.StatusCode, string(respBody), err)
	}
	return nil
}

func (c *ServiceAlertsClient) createNotification(product, subject, serviceInstanceID, content string) SpaceNotificationRequest {
	emailBody := fmt.Sprintf("Alert from %s", product)
	if serviceInstanceID != "" {
		emailBody = fmt.Sprintf("%s, service instance %s", emailBody, serviceInstanceID)
	}
	emailBody = fmt.Sprintf("%s:\n\n%s", emailBody, content)

	return SpaceNotificationRequest{
		KindID:  DummyKindID,
		Subject: fmt.Sprintf("[Service Alert][%s] %s", product, subject),
		Text:    emailBody,
		ReplyTo: c.config.NotificationTarget.ReplyTo,
	}
}

func (c *ServiceAlertsClient) obtainUAAToken() (string, error) {
	errs := func(err error) (string, error) {
		return "", err
	}

	uaaURL, err := joinURL(c.config.NotificationTarget.Authentication.UAA.URL, "/oauth/token")
	if err != nil {
		return errs(err)
	}
	uaaTokenReq, err := http.NewRequest("POST", uaaURL, strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return errs(err)
	}
	uaaTokenReq.SetBasicAuth(c.config.NotificationTarget.Authentication.UAA.ClientID, c.config.NotificationTarget.Authentication.UAA.ClientSecret)
	uaaTokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	uaaTokenResp, err := c.httpClient.Do(uaaTokenReq)
	if err != nil {
		return errs(err)
	}
	defer uaaTokenResp.Body.Close()
	if uaaTokenResp.StatusCode != http.StatusOK {
		respBody, err := ioutil.ReadAll(uaaTokenResp.Body)
		return errs(fmt.Errorf("UAA expected to return HTTP 200, got %d. Body: %s%s\n", uaaTokenResp.StatusCode, string(respBody), err))
	}
	var uaaTokenRespBody UAATokenResponse
	if err := json.NewDecoder(uaaTokenResp.Body).Decode(&uaaTokenRespBody); err != nil {
		return errs(err)
	}
	return uaaTokenRespBody.Token, nil
}

func joinURL(base, urlPath string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, urlPath)
	return u.String(), nil
}
