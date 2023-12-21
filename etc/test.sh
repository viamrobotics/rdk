#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"
cd $ROOT_DIR

# Race is unsupported on some linux/arm64 hosts. See https://github.com/golang/go/issues/29948.
# To run without race, use `make test-no-race` or `make test-go-no-race`.
# Running race and cover at the same time results in DRAMATIC test slowdowns, especially with parallel processing.

if [[ "$1" == "cover" ]]; then
	COVER=-coverprofile=coverage.txt
fi

if [[ "$1" == "race" ]]; then
	RACE=-race
	LOGFILE="--jsonfile json.log"
fi

# We run analyzetests on every run, pass or fail. We only run analyzecoverage when all tests passed.
PION_LOG_WARN=webrtc,datachannel,sctp gotestsum --format standard-verbose $LOGFILE -- -tags=no_skip $RACE $COVER ./...
SUCCESS=$?

if [[ $RACE != "" ]]; then
	cat json.log | go run ./etc/analyzetests/main.go
	exit $?
fi

if [ "$SUCCESS" != "0" ]; then
	exit 1
fi

if [[ $COVER != "" ]]; then
	cat coverage.txt | go run ./etc/analyzecoverage/main.go
fi
