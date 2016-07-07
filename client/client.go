package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

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

	httpClient := herottp.New(herottp.Config{Timeout: time.Second * 30, DisableTLSCertificateVerification: skipSSLValidation})
	return &ServiceAlertsClient{config: config, httpClient: httpClient}
}

func (c *ServiceAlertsClient) SendServiceAlert(product, subject, serviceInstanceID, content string) error {
	spaceGUID, err := c.obtainSpaceGUID()
	if err != nil {
		return err
	}

	token, err := c.obtainNotificationsClientToken()
	if err != nil {
		return err
	}
	notificationRequest, err := c.createNotification(product, subject, serviceInstanceID, content)
	if err != nil {
		return err
	}
	return c.sendNotification(token, notificationRequest, spaceGUID)
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
		return err
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
		return errs(err)
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

func (c *ServiceAlertsClient) sendCFApiRequest(uaaToken string, apiRequest CFApiRequest) ([]byte, error) {
	apiRequestURL, err := joinURL(c.config.CloudController.URL, apiRequest.Path, apiRequest.Filter)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", apiRequestURL, bytes.NewReader([]byte{'{', '}'}))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", uaaToken))

	apiResponse, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	apiResponseBody, err := ioutil.ReadAll(apiResponse.Body)

	return apiResponseBody, err
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

func (c *ServiceAlertsClient) obtainGUIDUsingRequest(token string, request CFApiRequest) (string, error) {
	responseBody, err := c.sendCFApiRequest(token, request)
	if err != nil {
		return "", err
	}

	resource, err := unmarshalCFResponse(responseBody)
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

func unmarshalCFResponse(body []byte) (response CFResourcesResponse, err error) {
	err = json.Unmarshal(body, &response)
	if err != nil {
		return CFResourcesResponse{}, fmt.Errorf("CF response not parseable: %s", err.Error())
	}
	return
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
