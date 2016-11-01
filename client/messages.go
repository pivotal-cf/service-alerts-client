package client

const DummyKindID = "service-alerts"

type CFApiRequest struct {
	Path   string
	Filter string
}

type CFResourcesResponse struct {
	TotalResults int         `json:"total_results"`
	Resources    CFResources `json:"resources"`
}

type CFResources []CFResource

type CFResource struct {
	Metadata CFMetadata `json:"metadata"`
}

type CFMetadata struct {
	GUID string `json:"guid"`
}

type SpaceNotificationRequest struct {
	KindID  string `json:"kind_id"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	ReplyTo string `json:"reply_to,omitempty"`
}

type UAATokenResponse struct {
	Token string `json:"access_token"`
}

type CFInfoResponse struct {
	UAAUrl string `json:"token_endpoint"`
}
