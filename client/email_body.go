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
