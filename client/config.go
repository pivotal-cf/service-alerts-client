package client

type Config struct {
	CloudController      CloudController    `yaml:"cloud_controller"`
	NotificationTarget   NotificationTarget `yaml:"notification_target"`
	RetryTimeoutSeconds  int                `yaml:"retry_timeout_seconds"`
	GlobalTimeoutSeconds int                `yaml:"timeout_seconds"`
}

type CloudController struct {
	URL      string `yaml:"url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type NotificationTarget struct {
	URL               string         `yaml:"url"`
	SkipSSLValidation *bool          `yaml:"skip_ssl_validation"`
	CFOrg             string         `yaml:"cf_org"`
	CFSpace           string         `yaml:"cf_space"`
	ReplyTo           string         `yaml:"reply_to"`
	Authentication    Authentication `yaml:"authentication"`
}

type Authentication struct {
	UAA UAA `yaml:"uaa"`
}

type UAA struct {
	URL          string `yaml:"url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}
