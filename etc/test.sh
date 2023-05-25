#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"
cd $ROOT_DIR

if [[ "$1" == "cover" ]]; then
	COVER=-coverprofile=coverage.txt
fi

# We run analyzetests on every run, pass or fail. We only run analyzecoverage when all tests passed.
gotestsum --format standard-verbose --jsonfile json.log -- -timeout 20m -tags=no_skip -race $COVER ./...
SUCCESS=$?

cat json.log | go run ./etc/analyzetests/main.go

if [ "$SUCCESS" != "0" ]; then
	exit 1
fi

cat coverage.txt | go run ./etc/analyzecoverage/main.go
