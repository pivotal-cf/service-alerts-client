#!/usr/bin/env bash

set -euox pipefail

usage() {
  echo "Usage: $(basename "$0") [bosh-lite-env-name]"
  exit 1
}

if [ $# -ne 1 ] ; then
  usage
fi

env_name="$1"
export MYSQL_SERVER="$(lpass show 1168984317182369735 --password)"
export BOSH_DEPLOYMENT_VARS="/tmp/broker-${env_name}.yml"
export BOSH_VARS_STORE="/Users/pivotal/workspace/services-enablement-lites-pool/bosh-vars/${env_name}.yml"
$META/concourse/bosh-lites-pool/tasks/make-broker-deployment-vars.sh > "$BOSH_DEPLOYMENT_VARS"

uaac target https://uaa.$BOSH_LITE_DOMAIN --skip-ssl-validation
uaac token client get admin -s $(credhub get -n /bosh-lite/cf/uaa_admin_client_secret -j | jq '.value' -r)

#Add notifications-admin client
notifications_client=notifications-admin
notification_client_secret=supersecretbrokerpassword

set +e
uaac client get $notifications_client
exit_code=$?

set -e

if [[ $exit_code -ne 0 ]]; then
  echo "uaac client ${notifications_client} not found. creating client..."
  uaac client add $notifications_client \
    --authorized_grant_types client_credentials \
    --authorities notifications.manage,notifications.write,notification_templates.write,notification_templates.read,critical_notifications.write,scim.read,cloud_controller.admin \
    --secret $notification_client_secret \
    --no-interactive
fi
#Upload notifications-release
bosh upload-release "https://github.com/cloudfoundry-incubator/notifications-release/releases/download/v57/notifications-57.tgz"

#
#cf api $CF_API --skip-ssl-validation
#cf auth
#cf target -o system -s system
#
#echo "delete existing go buildpack"
#cf delete-buildpack go_buildpack -f
#
#echo "creating custom go buildpack"
#cf create-buildpack go_buildpack https://github.com/cloudfoundry/go-buildpack/releases/download/v1.8.34/go-buildpack-cflinuxfs3-v1.8.34.zip 6 --enable
#
#cf buildpacks


#Deploy notifications
bosh -d notifications deploy -n $META/concourse/service-alerts-client/scripts/operations/notifications.yml \
  --vars-file="$BOSH_DEPLOYMENT_VARS" \
  --var mysqlserver="$MYSQL_SERVER" \
  --var system_domain="$BOSH_LITE_DOMAIN" \
  --var uaa_admin_client_secret="$(credhub get -n /bosh-lite/cf/uaa_admin_client_secret -j | jq '.value' -r)"

#Register notifications with cloudfoundry
bosh -d notifications run-errand deploy-notifications
