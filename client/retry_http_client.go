// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/cenk/backoff"
	"github.com/craigfurman/herottp"
)

const httpClientTimeout = 30 * time.Second

type RetryHTTPClient struct {
	config     Config
	httpClient *herottp.Client
	logger     *log.Logger
}

func NewRetryHTTPClient(config Config, logger *log.Logger) *RetryHTTPClient {
	skipSSLValidation := false
	if config.SkipSSLValidation != nil {
		skipSSLValidation = *config.SkipSSLValidation
	}

	httpClient := herottp.New(herottp.Config{
		Timeout: httpClientTimeout,
		DisableTLSCertificateVerification: skipSSLValidation,
	})

	return &RetryHTTPClient{config: config, httpClient: httpClient, logger: logger}
}

func (r *RetryHTTPClient) doRequestWithRetries(label string, req *http.Request) (*http.Response, error) {
	var apiResponse *http.Response

	retryRequest := func() error {
		var networkErr error

		apiResponse, networkErr = r.httpClient.Do(req)
		if networkErr != nil {
			return HTTPRequestError{error: networkErr, config: r.config}
		}

		if retryableResponse(apiResponse) {
			return HTTPRequestError{
				error:  fmt.Errorf("%s expected to return HTTP 200, got %d. %s", label, apiResponse.StatusCode, responseBodyDetails(apiResponse)),
				config: r.config,
			}
		}

		return nil
	}

	retryError := backoff.RetryNotify(retryRequest, r.buildExponentialBackoff(), r.buildRetryLogging(label))
	if retryError != nil {
		r.logger.Printf("Giving up, %s request failed: %s", label, retryError)
		return nil, retryError
	}

	if apiResponse.StatusCode != http.StatusOK {
		failStatusCodeErr := fmt.Errorf("%s expected to return HTTP 200, got %d. %s", label, apiResponse.StatusCode, responseBodyDetails(apiResponse))
		return nil, failStatusCodeErr
	}

	return apiResponse, nil
}

func (r *RetryHTTPClient) buildRetryLogging(label string) func(err error, next time.Duration) {
	return func(err error, next time.Duration) {
		r.logger.Printf("Retrying in %d seconds, %s request error: %s", int(next.Seconds()), label, err)
	}
}

func (r *RetryHTTPClient) buildExponentialBackoff() *backoff.ExponentialBackOff {
	exponentialBackoff := backoff.NewExponentialBackOff()

	maxElapsedTime := defaultGlobalTimeout
	if r.config.GlobalTimeoutSeconds != 0 {
		maxElapsedTime = time.Duration(r.config.GlobalTimeoutSeconds) * time.Second
	}

	exponentialBackoff.InitialInterval = 1 * time.Second
	exponentialBackoff.RandomizationFactor = 0
	exponentialBackoff.Multiplier = 2
	exponentialBackoff.MaxInterval = 16 * time.Second
	exponentialBackoff.MaxElapsedTime = maxElapsedTime

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
