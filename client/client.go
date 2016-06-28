package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
	req.Header.Set("Authorization", "Bearer GET_ME_FROM_UAA")
	req.Header.Set("Content-Type", "application/json")

	httpClient := herottp.New(herottp.Config{Timeout: time.Second * 30})
	_, err = httpClient.Do(req)
	return err
}
