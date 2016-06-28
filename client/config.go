package client

type Config struct {
	NotificationTarget NotificationTarget `yaml:"notification_target"`
}

type NotificationTarget struct {
	URL            string         `yaml:"url"`
	CFSpaceGUID    string         `yaml:"cf_space_guid"`
	ReplyTo        string         `yaml:"reply_to"`
	Authentication Authentication `yaml:"authentication"`
}

type Authentication struct {
	UAA UAA `yaml:"uaa"`
}

type UAA struct {
	URL          string `yaml:"url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}
