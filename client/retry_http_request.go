package client

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cenk/backoff"
)

const defaultRetryTimeout = 30 * time.Second

func (c *ServiceAlertsClient) doRequestWithRetries(label string, req *http.Request) (*http.Response, error) {
	var apiResponse *http.Response

	retryRequest := func() error {
		var networkErr error

		apiResponse, networkErr = c.httpClient.Do(req)
		if networkErr != nil {
			return HTTPRequestError{error: networkErr, config: c.config}
		}

		if retryableResponse(apiResponse) {
			return HTTPRequestError{
				error:  fmt.Errorf("%s expected to return HTTP 200, got %d. %s", label, apiResponse.StatusCode, responseBodyDetails(apiResponse)),
				config: c.config,
			}
		}

		return nil
	}

	retryError := backoff.RetryNotify(retryRequest, c.buildExponentialBackoff(), c.buildRetryLogging(label))
	if retryError != nil {
		c.logger.Printf("Giving up, %s request failed: %s", label, retryError)
		return nil, retryError
	}

	if apiResponse.StatusCode != http.StatusOK {
		failStatusCodeErr := fmt.Errorf("%s expected to return HTTP 200, got %d. %s", label, apiResponse.StatusCode, responseBodyDetails(apiResponse))
		return nil, failStatusCodeErr
	}

	return apiResponse, nil
}

func (c *ServiceAlertsClient) buildRetryLogging(label string) func(err error, next time.Duration) {
	return func(err error, next time.Duration) {
		c.logger.Printf("Retrying in %d seconds, %s request error: %s", int(next.Seconds()), label, err)
	}
}

func (c *ServiceAlertsClient) buildExponentialBackoff() *backoff.ExponentialBackOff {
	exponentialBackoff := backoff.NewExponentialBackOff()

	retryTimeout := defaultRetryTimeout
	if c.config.RetryTimeoutSeconds != 0 {
		retryTimeout = time.Duration(c.config.RetryTimeoutSeconds) * time.Second
	}

	exponentialBackoff.InitialInterval = 1 * time.Second
	exponentialBackoff.RandomizationFactor = 0
	exponentialBackoff.Multiplier = 2
	exponentialBackoff.MaxInterval = 16 * time.Second
	exponentialBackoff.MaxElapsedTime = retryTimeout

	return exponentialBackoff
}

func responseBodyDetails(response *http.Response) string {
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	var details string
	if err != nil {
		details = fmt.Sprintf("Read body error: %s", err.Error())
	} else {
		details = fmt.Sprintf("Body: %s", string(body))
	}

	return details
}

func retryableResponse(apiResponse *http.Response) bool {
	return apiResponse.StatusCode >= http.StatusInternalServerError ||
		(apiResponse.StatusCode == http.StatusNotFound && apiResponse.Header.Get("X-Cf-Routererror") == "unknown_route")
}
