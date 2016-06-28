#!/bin/bash -eu

(cd $(dirname $0)/.. && ginkgo -randomizeSuites=true -randomizeAllSpecs=true -keepGoing=true -r "$@")
