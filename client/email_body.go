// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package client

import (
	"bytes"
	"text/template"
	"time"
)

var emailTemplateText = `Alert from {{.Product}}{{if .ServiceInstanceID}}, service instance {{.ServiceInstanceID}}{{end}}:

{{.Content}}

[Alert generated at {{.Timestamp}}]`
var emailTemplate = template.Must(template.New("emailBody").Parse(emailTemplateText))

func templateEmailBody(product, serviceInstanceID, content string, t time.Time) (string, error) {
	var buffer bytes.Buffer
	data := struct {
		Product           string
		ServiceInstanceID string
		Content           string
		Timestamp         string
	}{
		product,
		serviceInstanceID,
		content,
		t.Format(time.RFC3339),
	}
	if err := emailTemplate.Execute(&buffer, data); err != nil {
		return "", err
	}
	return buffer.String(), nil
}
