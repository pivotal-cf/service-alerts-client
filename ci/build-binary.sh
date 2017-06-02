#! /bin/bash -e

# Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
# This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

client_git() {
  git -C "$CLIENT_DIR" "$@"
}

must_have() {
  if [[ -z "${!1:-}" ]]
    then echo "must have environment variable $1"
    return 1
  fi
}

IMPORT_PATH=github.com/pivotal-cf/service-alerts-client
CLIENT_DIR=$PWD/src/$IMPORT_PATH
BINARY_VERSION="$(client_git tag --list 'v*' --contains HEAD | sed -n 's/^v//p')"
must_have BINARY_VERSION

export GOPATH=$PWD
CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o bin/service-alerts-client-$BINARY_VERSION $IMPORT_PATH/cmd/send-service-alert
