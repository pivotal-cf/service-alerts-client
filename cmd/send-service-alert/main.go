// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/pivotal-cf/service-alerts-client/client"

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

	logFlags := log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC
	logger := log.New(os.Stderr, "[service alerts client] ", logFlags)

	alertsClient := client.New(config, logger)
	clientErr := alertsClient.SendServiceAlert(*product, *subject, *serviceInstanceID, *content)
	if clientErr != nil {
		switch clientErr.(type) {
		case client.HTTPRequestError:
			logger.Println(clientErr.(client.HTTPRequestError).ErrorMessageForUser())
			os.Exit(2)
		default:
			mustNot(clientErr)
		}
	}
}

func mustNot(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

var must = mustNot
