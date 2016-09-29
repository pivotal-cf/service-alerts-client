package main

import (
	"log"
	"os"

	"github.com/pivotal-cf/service-alerts-client/client"
)

var (
	skipSSL = true

	cloudController = client.CloudController{
		URL:      "https://api.<CF_DOMAIN>",
		User:     "<USERNAME>",
		Password: "<PASSWORD>",
	}

	auth = client.Authentication{
		UAA: client.UAA{
			URL:          "https://uaa.<CF_DOMAIN>",
			ClientID:     "<CLIENTID>",
			ClientSecret: "<CLIENT_SECRET>",
		},
	}

	notificationTarget = client.NotificationTarget{
		URL:               "https://notifications.<CF_DOMAIN>",
		CFOrg:             "<ORG>",
		CFSpace:           "<SPACE>", // Org and space where the cf-notifications service is running
		SkipSSLValidation: &skipSSL,
		Authentication:    auth,
	}
)

func main() {

	config := client.Config{
		CloudController:    cloudController,
		NotificationTarget: notificationTarget,
	}

	logFlags := log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC
	logger := log.New(os.Stdout, "[example] ", logFlags)
	alertClient := client.New(config, logger)
	err := alertClient.SendServiceAlert("product", "subject", "serviceInstanceID", "content")
	if err != nil {
		logger.Fatalf("Failed to do anything at all :( ... %s/n", err)
	}

	logger.Printf("Successfully posted notification to CF notification service for org: %s, space: %s", config.NotificationTarget.CFOrg, config.NotificationTarget.CFSpace)

}
