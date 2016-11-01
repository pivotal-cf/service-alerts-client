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
