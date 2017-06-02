// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

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
