# Service Alerts Client

Service Alerts Client allows service authors to send alert emails via the [Cloud Foundry Notifications Service](https://github.com/cloudfoundry-incubator/notifications) to all users with the SpaceDeveloper role in the configured Cloud Foundry space.

# Installation
Clone [the git repository](https://github.com/pivotal-cf/service-alerts-client) at `$GOPATH/src/github.com/pivotal-cf` and compile it with `go install github.com/pivotal-cf/service-alerts-client/cmd/send-service-alert`.

# Cloud Foundry set up
The tool requires:
- A CF user (with SpaceAuditor role as a minimum) to query the CF API.
- A UAA client (with the `notifications.write` authority as a minimum) to invoke the CF notifications service.

Here is an example setup:

## CF space auditor
We recommend creating a CF user with least privilege to query the CF API and provide their credentials in the config file. They need to have a space role in the configured space:

```
cf target -o CF_ORG -s CF_SPACE
cf create-user SPACE_AUDITOR_USERNAME SPACE_AUDITOR_PASSWORD
cf set-space-role SPACE_AUDITOR_USERNAME CF_ORG CF_SPACE SpaceAuditor
```

## Service alert recipient
Only CF users with the space developer role in the configured space will receive emails.

```
cf set-space-role SPACE_DEV_USERNAME CF_ORG CF_SPACE SpaceDeveloper
```

## UAA client
We recommend creating a UAA client with the least privilege to send notifications to a space.
A full list of related permissions can be found in the [CF notifications service documentation](https://github.com/cloudfoundry-incubator/notifications#send-notifications).

```
uaac client add CLIENT_NAME \
  --secret CLIENT_SECRET \
  --authorized_grant_types client_credentials \
  --authorities notifications.write
```

# Usage

## As a library

There is an example go program that calls the service-alerts-client [here](https://github.com/pivotal-cf/service-alerts-client/blob/master/realservicetests/example/main.go).
The config values are redacted so ensure to fill with values of your set up. Make sure the space you use has a user with an email address.

## On the command line
```
send-service-alert \
  -config <config file path> \
  -product <product name> \
  -service-instance <OPTIONAL: service instance ID> \
  -subject <email subject> \
  -content <email content>
```

The format of the config file:

```yaml
cloud_controller:
  url: <Cloud Foundry API URL>
  user: <Cloud Foundry username with SpaceAuditor role in cf_space>
  password: <Cloud Foundry password>
notifications:
  service_url: <Cloud Foundry notification service URL>
  cf_org: <Cloud Foundry org name>
  cf_space: <Cloud Foundry space name>
  reply_to: <OPTIONAL: email reply-to address. This is required for some SMTP servers>
  client_id: <UAA client ID with authorities to send notifications>
  client_secret: <UAA client secret>
timeout_seconds: <OPTIONAL: default is 60>
skip_ssl_validation: <OPTIONAL: ignore TLS certification verification errors>
```

## HTTP retry strategy

HTTP requests will be retried if they fail due to a network error, a response status code of 5xx, or 404 from the Cloud Foundry Router. HTTP requests will be attempted with exponential back-off between attempts.

The timeout refers to a global maximum time to send a service alert, not a timeout per HTTP request.

# Email content

The email that is sent will have the subject `CF Notification: [Service Alert][<product>] <subject>`. The body is plain text only.

When the `service-instance` flag is set, the body will be in the following format:

```
You received this message because you belong to the "<org>" space in the "<space>" organization.
Alert from <product>, service instance <service instance ID>:

<content>

[Alert generated at <RFC 3339 datetime>]
```

When `service-instance` flag is not set, the body will be in the following format:

```
Alert from <product>:

<content>

[Alert generated at <RFC 3339 datetime>]
```
