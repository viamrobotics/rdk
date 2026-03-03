#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"
cd $ROOT_DIR

# If SKIP_ROBOT_IMPL is set (e.g. by CI when robot/impl deps are unchanged),
# build the test target as all packages except robot/impl.
if [[ -n "$SKIP_ROBOT_IMPL" && -z "$TEST_TARGET" ]]; then
	echo "SKIP_ROBOT_IMPL is set; excluding go.viam.com/rdk/robot/impl from test run"
	TEST_TARGET=$(go list ./... | grep -v '^go.viam.com/rdk/robot/impl$')
fi
TEST_TARGET=${TEST_TARGET:-./...}

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

FORMAT='standard-verbose'
if test -n "$GITHUB_RUN_ID"; then
	FORMAT='github-actions'
    FORMAT='standard-quiet'
fi

# We run analyzetests on every run, pass or fail. We only run analyzecoverage when all tests passed.
PION_LOG_WARN=webrtc,datachannel,sctp gotestsum --format $FORMAT $LOGFILE -- -tags=no_skip -timeout 40m $RACE $COVER $TEST_TARGET
SUCCESS=$?

if [[ $RACE != "" ]]; then
	cat json.log | go run ./etc/analyzetests/main.go
	if [ "$?" != "0" ]; then
		exit 1
	fi
fi

if [ "$SUCCESS" != "0" ]; then
	exit 1
fi

if [[ $COVER != "" ]]; then
	cat coverage.txt | go run ./etc/analyzecoverage/main.go
fi
