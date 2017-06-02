// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

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

	Notifications = client.Notifications{
		ServiceURL:   "https://notifications.<CF_DOMAIN>",
		CFOrg:        "<ORG>",
		CFSpace:      "<SPACE>", // Org and space where the cf-notifications service is running
		ClientID:     "<CLIENTID>",
		ClientSecret: "<CLIENT_SECRET>",
	}
)

func main() {

	config := client.Config{
		CloudController:   cloudController,
		Notifications:     Notifications,
		SkipSSLValidation: &skipSSL,
	}

	logFlags := log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC
	logger := log.New(os.Stdout, "[example] ", logFlags)
	alertClient := client.New(config, logger)
	err := alertClient.SendServiceAlert("product", "subject", "serviceInstanceID", "content")
	if err != nil {
		logger.Fatalf("Failed to do anything at all :( ... %s/n", err)
	}

	logger.Printf("Successfully posted notification to CF notification service for org: %s, space: %s", config.Notifications.CFOrg, config.Notifications.CFSpace)

}
