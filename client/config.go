package client

type Config struct {
	CloudController      CloudController `yaml:"cloud_controller"`
	Notifications        Notifications   `yaml:"notifications"`
	GlobalTimeoutSeconds int             `yaml:"timeout_seconds"`
	SkipSSLValidation    *bool           `yaml:"skip_ssl_validation"`
}

type CloudController struct {
	URL      string `yaml:"url"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type Notifications struct {
	ServiceURL   string `yaml:"service_url"`
	CFOrg        string `yaml:"cf_org"`
	CFSpace      string `yaml:"cf_space"`
	ReplyTo      string `yaml:"reply_to"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}
