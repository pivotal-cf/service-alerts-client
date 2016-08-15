package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/concourse/retryhttp"
)

func (c *ServiceAlertsClient) SendServiceAlert(product, subject, serviceInstanceID, content string) error {
	errChan := make(chan error)
	go c.sendServiceAlert(product, subject, serviceInstanceID, content, errChan)
	select {
	case err := <-errChan:
		return err
	case <-time.After(requestTimeout):
		return HTTPRequestError{error: errors.New("sending service alert timed out"), config: c.config}
	}
}

func (c *ServiceAlertsClient) sendServiceAlert(product, subject, serviceInstanceID, content string, errChan chan<- error) {
	spaceGUID, err := c.obtainSpaceGUID()
	if err != nil {
		errChan <- err
		return
	}

	token, err := c.obtainNotificationsClientToken()
	if err != nil {
		errChan <- err
		return
	}
	notificationRequest, err := c.createNotification(product, subject, serviceInstanceID, content)
	if err != nil {
		errChan <- err
		return
	}

	errChan <- c.sendNotification(token, notificationRequest, spaceGUID)
}

func (c *ServiceAlertsClient) sendNotification(uaaToken string, notificationRequest SpaceNotificationRequest, spaceGUID string) error {
	reqBytes, err := json.Marshal(notificationRequest)
	if err != nil {
		return err
	}

	sendNotificationRequestURL, err := joinURL(c.config.NotificationTarget.URL, fmt.Sprintf("/spaces/%s", spaceGUID), "")
	req, err := http.NewRequest("POST", sendNotificationRequestURL, bytes.NewReader(reqBytes))
	if err != nil {
		return err
	}

	req.Header.Set("X-NOTIFICATIONS-VERSION", "1")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uaaToken))
	req.Header.Set("Content-Type", "application/json")

	sendNotificationResponse, err := c.httpClient.Do(req)
	if err != nil {
		return HTTPRequestError{error: err, config: c.config}
	}

	if sendNotificationResponse.StatusCode != http.StatusOK {
		respBody, err := ioutil.ReadAll(sendNotificationResponse.Body)
		return fmt.Errorf("CF Notifications service expected to return HTTP 200, got %d. Body: %s%s\n", sendNotificationResponse.StatusCode, string(respBody), err)
	}
	return nil
}

func (c *ServiceAlertsClient) createNotification(product, subject, serviceInstanceID, content string) (SpaceNotificationRequest, error) {
	textBody, err := templateEmailBody(product, serviceInstanceID, content, time.Now())
	if err != nil {
		return SpaceNotificationRequest{}, err
	}

	return SpaceNotificationRequest{
		KindID:  DummyKindID,
		Subject: fmt.Sprintf("[Service Alert][%s] %s", product, subject),
		Text:    textBody,
		ReplyTo: c.config.NotificationTarget.ReplyTo,
	}, nil
}

func (c *ServiceAlertsClient) obtainNotificationsClientToken() (string, error) {
	return c.obtainUAAToken(c.config.NotificationTarget.Authentication.UAA.ClientID, c.config.NotificationTarget.Authentication.UAA.ClientSecret, "client_credentials")
}

func (c *ServiceAlertsClient) obtainCFUserToken() (string, error) {
	return c.obtainUAAToken(c.config.CloudController.User, c.config.CloudController.Password, "password")
}

func (c *ServiceAlertsClient) obtainUAAToken(username, password, grantType string) (string, error) {
	uaaTokenReq, err := c.constructRequestForGrantType(username, password, grantType)
	if err != nil {
		return errs(err)
	}

	uaaTokenResp, err := c.httpClient.Do(uaaTokenReq)
	if err != nil {
		return errs(HTTPRequestError{error: err, config: c.config})
	}
	defer uaaTokenResp.Body.Close()
	if uaaTokenResp.StatusCode != http.StatusOK {
		respBody, err := ioutil.ReadAll(uaaTokenResp.Body)
		return errs(fmt.Errorf("UAA expected to return HTTP 200, got %d. Body: %s%s\n", uaaTokenResp.StatusCode, string(respBody), err))
	}
	var uaaTokenRespBody UAATokenResponse
	if err := json.NewDecoder(uaaTokenResp.Body).Decode(&uaaTokenRespBody); err != nil {
		return errs(fmt.Errorf("UAA response not parseable: %s", err.Error()))
	}
	return uaaTokenRespBody.Token, nil
}

func (c *ServiceAlertsClient) constructRequestForGrantType(username, password, grantType string) (*http.Request, error) {
	uaaURL, err := joinURL(c.config.NotificationTarget.Authentication.UAA.URL, "/oauth/token", "")
	if err != nil {
		return nil, err
	}

	var postBody string
	if grantType == "password" {
		postBody = fmt.Sprintf("grant_type=password&username=%s&scope=&password=%s", username, password)
	} else {
		postBody = "grant_type=client_credentials"
	}

	uaaTokenReq, err := http.NewRequest("POST", uaaURL, strings.NewReader(postBody))
	if err != nil {
		return nil, err
	}

	if grantType == "password" {
		// Special header required to obtain a token using a CF user's credentials
		uaaTokenReq.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte("cf:"))))
	} else {
		uaaTokenReq.SetBasicAuth(username, password)
	}

	uaaTokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return uaaTokenReq, nil
}

func (c *ServiceAlertsClient) sendCFApiRequest(uaaToken string, apiRequest CFApiRequest) (*http.Response, error) {
	apiRequestURL, err := joinURL(c.config.CloudController.URL, apiRequest.Path, apiRequest.Filter)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", apiRequestURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uaaToken))

	apiResponse, err := c.httpClient.Do(req)
	if err != nil {
		return nil, HTTPRequestError{error: err, config: c.config}
	}

	return apiResponse, nil
}

func (c *ServiceAlertsClient) obtainSpaceGUID() (string, error) {
	cfUserToken, err := c.obtainCFUserToken()
	if err != nil {
		return errs(err)
	}

	getOrganisationRequest := c.createOrgQueryRequest()
	orgGUID, err := c.obtainGUIDUsingRequest(cfUserToken, getOrganisationRequest)
	if err != nil {
		return errs(formattedCFError("org", c.config.NotificationTarget.CFOrg, err))
	}

	getSpaceRequest := c.createSpaceQueryRequest(orgGUID)
	spaceGUID, err := c.obtainGUIDUsingRequest(cfUserToken, getSpaceRequest)
	if err != nil {
		return errs(formattedCFError("space", c.config.NotificationTarget.CFSpace, err))
	}

	return spaceGUID, nil
}

func (c *ServiceAlertsClient) createOrgQueryRequest() CFApiRequest {
	orgQueryRequest := CFApiRequest{
		Path:   "/v2/organizations",
		Filter: fmt.Sprintf("name:%s", c.config.NotificationTarget.CFOrg),
	}
	return orgQueryRequest
}

func (c *ServiceAlertsClient) createSpaceQueryRequest(orgGUID string) CFApiRequest {
	spaceQueryRequest := CFApiRequest{
		Path:   fmt.Sprintf("/v2/organizations/%s/spaces", orgGUID),
		Filter: fmt.Sprintf("name:%s", c.config.NotificationTarget.CFSpace),
	}
	return spaceQueryRequest
}

type sleeper struct{}

func (sleeper) Sleep(duration time.Duration) {
	time.Sleep(duration)
}

func (c *ServiceAlertsClient) obtainGUIDUsingRequest(token string, request CFApiRequest) (string, error) {
	var response *http.Response
	var err error
	retryhttp.Retry(c.logger, retryhttp.ExponentialRetryPolicy{Timeout: requestTimeout}, sleeper{}, func() bool {
		response, err = c.sendCFApiRequest(token, request)
		if err != nil {
			return false
		}
		if response.StatusCode != http.StatusOK {
			err = HTTPRequestError{error: fmt.Errorf("expected status 200, got %d", response.StatusCode), config: c.config}
			return false
		}

		err = nil
		return true
	})

	if err != nil {
		return "", err
	}

	resource, err := unmarshalCFResponse(response.Body)
	if err != nil {
		return "", err
	}

	if resource.TotalResults == 0 {
		return "", CFResourceNotFound{error: fmt.Errorf("CF resource not found")}
	}

	return resource.Resources[0].Metadata.GUID, nil
}

func formattedCFError(cfResourceType, cfResourceName string, err error) error {
	switch err := err.(type) {
	case CFResourceNotFound:
		return fmt.Errorf("CF %s not found: '%s'", cfResourceType, cfResourceName)
	default:
		return err
	}
}

func unmarshalCFResponse(body io.ReadCloser) (CFResourcesResponse, error) {
	defer body.Close()
	var response CFResourcesResponse
	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return CFResourcesResponse{}, fmt.Errorf("CF response not parseable: %s", err.Error())
	}
	return response, nil
}

type CFResourceNotFound struct {
	error
}

func joinURL(base, urlPath, filter string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, urlPath)
	if filter != "" {
		q := u.Query()
		q.Set("q", filter)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func errs(err error) (string, error) {
	return "", err
}
