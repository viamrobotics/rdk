#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"
cd $ROOT_DIR

# Race is unsupported on some linux/arm64 hosts. See https://github.com/golang/go/issues/29948.
# To run without race, use `make test-no-race` or `make test-go-no-race`.

if [[ "$1" == "cover-with-race" ]]; then
	COVER=-coverprofile=coverage.txt
	RACE=-race
fi

if [[ "$1" == "race" ]]; then
	RACE=-race
fi

# We run analyzetests on every run, pass or fail. We only run analyzecoverage when all tests passed.
PION_LOG_WARN=webrtc,datachannel,sctp gotestsum --format standard-verbose --jsonfile json.log -- -tags=no_skip $RACE $COVER ./...
SUCCESS=$?

cat json.log | go run ./etc/analyzetests/main.go

if [ "$SUCCESS" != "0" ]; then
	exit 1
fi

cat coverage.txt | go run ./etc/analyzecoverage/main.go
