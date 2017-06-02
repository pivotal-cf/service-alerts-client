// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

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
