package client

import (
	"fmt"
	"os"
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

func New(config Config) *ServiceAlertsClient {
	skipSSLValidation := false
	if config.NotificationTarget.SkipSSLValidation != nil {
		skipSSLValidation = *config.NotificationTarget.SkipSSLValidation
	}

	logger := lager.NewLogger("service alerts client")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))

	retryTimeLimit := retryTimeLimit(config)

	httpClient := herottp.New(herottp.Config{Timeout: retryTimeLimit + successfulRequestTimeout, DisableTLSCertificateVerification: skipSSLValidation})
	roundTripper := httpClient.Client.Transport

	httpClient.Client.Transport = &retryhttp.RetryRoundTripper{
		Logger:       logger,
		Sleeper:      clock.NewClock(),
		RetryPolicy:  retryPolicy(retryTimeLimit),
		RoundTripper: roundTripper,
	}

	return &ServiceAlertsClient{config: config, httpClient: httpClient}
}

const successfulRequestTimeout = time.Second * 30
const defaultHTTPRetryTimeLimitSeconds = 60

func retryTimeLimit(config Config) time.Duration {
	seconds := config.HTTPRetryTimeLimitSeconds
	if seconds == 0 {
		seconds = defaultHTTPRetryTimeLimitSeconds
	}
	return time.Second * time.Duration(seconds)
}

func retryPolicy(timeLimit time.Duration) retryhttp.RetryPolicy {
	return retryhttp.ExponentialRetryPolicy{Timeout: timeLimit}
}

type HTTPRequestError struct {
	error
	config Config
}

func (n HTTPRequestError) ErrorMessageForUser() string {
	return fmt.Sprintf("failed to send notification to org: %s, space: %s", n.config.NotificationTarget.CFOrg, n.config.NotificationTarget.CFSpace)
}
