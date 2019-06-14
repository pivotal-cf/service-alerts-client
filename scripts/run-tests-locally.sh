#!/bin/bash -eu

# Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
# This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
set -oeu pipefail

usage() {
  echo "Usage: $(basename "$0") [bosh-lite-env-name] [--skip-deploy]"
  exit 1
}

if [[ $# -lt 1 ]] ; then
  usage
fi

export NOTIFICATIONS_CLIENT_ID="notifications-admin"
export NOTIFICATIONS_CLIENT_SECRET="supersecretbrokerpassword"
export MAILHOG_URL="http://35.195.81.151" # deployed on concourse bosh env
export CF_ADMIN_USERNAME="$CF_USERNAME"
export CF_ADMIN_PASSWORD="$CF_PASSWORD"
export NOTIFICATIONS_SERVICE_URL="https://notifications.${BOSH_LITE_DOMAIN}"


env_name="$1"
self="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
parent=`realpath $self/../`

if [[ $2 != "--skip-deploy" ]]; then
  ${self}/deploy-notification-on-lite.sh ${env_name}
fi

${parent}/scripts/real-service-tests.sh
