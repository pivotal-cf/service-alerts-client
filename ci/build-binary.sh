#! /bin/bash -e

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
RELEASE_VERSION="$(client_git tag --list 'v*' --contains HEAD | sed -n 's/^v//p')"
must_have RELEASE_VERSION

export GOPATH=$PWD
CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o bin/service-alerts-client-$RELEASE_VERSION $IMPORT_PATH/cmd/send-service-alert
