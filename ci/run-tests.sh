#! /bin/bash -eu

export GOPATH=$PWD
export PATH=$GOPATH/bin:$PATH

go get github.com/onsi/ginkgo/ginkgo

src/github.com/pivotal-cf/service-alerts-client/scripts/run-tests.sh
