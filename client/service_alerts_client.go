package client

import (
	"fmt"
	"log"
	"time"
)

type ServiceAlertsClient struct {
	config     Config
	httpClient *RetryHTTPClient
	logger     *log.Logger
}

const requestTimeout = 30 * time.Second

func New(config Config, logger *log.Logger) *ServiceAlertsClient {
	httpClient := NewRetryHTTPClient(config, logger)
	return &ServiceAlertsClient{config: config, httpClient: httpClient, logger: logger}
}

type HTTPRequestError struct {
	error
	config Config
}

func (n HTTPRequestError) ErrorMessageForUser() string {
	return fmt.Sprintf("failed to send notification to org: %s, space: %s", n.config.NotificationTarget.CFOrg, n.config.NotificationTarget.CFSpace)
}