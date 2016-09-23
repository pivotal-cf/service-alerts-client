package client

import (
	"fmt"
	"log"
	"time"

	"github.com/craigfurman/herottp"
)

type ServiceAlertsClient struct {
	config     Config
	httpClient *herottp.Client
	logger     *log.Logger
}

const requestTimeout = 30 * time.Second

func New(config Config, logger *log.Logger) *ServiceAlertsClient {
	skipSSLValidation := false
	if config.NotificationTarget.SkipSSLValidation != nil {
		skipSSLValidation = *config.NotificationTarget.SkipSSLValidation
	}

	httpClient := herottp.New(herottp.Config{
		Timeout: requestTimeout,
		DisableTLSCertificateVerification: skipSSLValidation,
	})

	return &ServiceAlertsClient{config: config, httpClient: httpClient, logger: logger}
}

type HTTPRequestError struct {
	error
	config Config
}

func (n HTTPRequestError) ErrorMessageForUser() string {
	return fmt.Sprintf("failed to send notification to org: %s, space: %s", n.config.NotificationTarget.CFOrg, n.config.NotificationTarget.CFSpace)
}
