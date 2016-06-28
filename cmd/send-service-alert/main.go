package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/pivotal-cf/service-alerts-client/client"

	"github.com/craigfurman/herottp"
	"gopkg.in/yaml.v2"
)

func main() {
	configFilePath := flag.String("config", "", "config file path")
	product := flag.String("product", "", "name of product")
	serviceInstanceID := flag.String("service-instance", "", "service instance ID (optional)")
	subject := flag.String("subject", "", "email subject")
	content := flag.String("content", "", "email body content")
	flag.Parse()

	configBytes, err := ioutil.ReadFile(*configFilePath)
	mustNot(err)

	var config client.Config
	must(yaml.Unmarshal(configBytes, &config))

	notificationsServiceReqBody := client.SpaceNotificationRequest{
		KindID:  "UPSERT_ME_TO_NOTIFICATION_SERVICE",
		Subject: fmt.Sprintf("[Service Alert][%s] %s", *product, *subject),
		Text:    fmt.Sprintf("Alert from %s, service instance %s:\n\n%s", *product, *serviceInstanceID, *content),
		ReplyTo: config.NotificationTarget.ReplyTo,
	}
	reqBytes, err := json.Marshal(notificationsServiceReqBody)
	mustNot(err)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/spaces/%s", config.NotificationTarget.URL, config.NotificationTarget.CFSpaceGUID), bytes.NewReader(reqBytes))
	mustNot(err)
	req.Header.Set("X-NOTIFICATIONS-VERSION", "1")
	req.Header.Set("Authorization", "Bearer GET_ME_FROM_UAA")
	req.Header.Set("Content-Type", "application/json")

	httpClient := herottp.New(herottp.Config{Timeout: time.Second * 30})
	httpClient.Do(req)
}

func mustNot(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

var must = mustNot
