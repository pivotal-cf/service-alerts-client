package client

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/retryhttp"

	"github.com/craigfurman/herottp"
)

type ServiceAlertsClient struct {
	config     Config
	httpClient *herottp.Client
}

const requestTimeout = time.Second * 60
const totalDelayBetweenAttempts = time.Second * 48 // Configures the policy to make 6 attempts with 31 seconds of delay between them

func New(config Config, logger lager.Logger) *ServiceAlertsClient {
	skipSSLValidation := false
	if config.NotificationTarget.SkipSSLValidation != nil {
		skipSSLValidation = *config.NotificationTarget.SkipSSLValidation
	}

	httpClient := herottp.New(herottp.Config{Timeout: requestTimeout, DisableTLSCertificateVerification: skipSSLValidation})
	roundTripper := httpClient.Client.Transport

	httpClient.Client.Transport = &retryhttp.RetryRoundTripper{
		Logger:       logger,
		Sleeper:      clock.NewClock(),
		RetryPolicy:  retryhttp.ExponentialRetryPolicy{Timeout: totalDelayBetweenAttempts},
		RoundTripper: roundTripper,
	}

	return &ServiceAlertsClient{config: config, httpClient: httpClient}
}

type HTTPRequestError struct {
	error
	config Config
}

func (n HTTPRequestError) ErrorMessageForUser() string {
	return fmt.Sprintf("failed to send notification to org: %s, space: %s", n.config.NotificationTarget.CFOrg, n.config.NotificationTarget.CFSpace)
}
