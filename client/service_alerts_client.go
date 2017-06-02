// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package client

import (
	"fmt"
	"log"
)

type ServiceAlertsClient struct {
	config     Config
	uaaUrl     string
	httpClient *RetryHTTPClient
	logger     *log.Logger
}

func New(config Config, logger *log.Logger) *ServiceAlertsClient {
	httpClient := NewRetryHTTPClient(config, logger)

	return &ServiceAlertsClient{config: config, httpClient: httpClient, logger: logger}
}

type HTTPRequestError struct {
	error
	config Config
}

func (n HTTPRequestError) ErrorMessageForUser() string {
	return fmt.Sprintf("failed to send notification to org: %s, space: %s", n.config.Notifications.CFOrg, n.config.Notifications.CFSpace)
}
