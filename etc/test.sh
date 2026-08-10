#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"
cd $ROOT_DIR

# If any SKIP_* vars are set by CI (when a package's dep tree is unchanged in
# the PR), build TEST_TARGET excluding those packages. See check-changes in
# .github/workflows/test.yml for how these flags are computed.
if [[ -z "$TEST_TARGET" ]] && [[ -n "${SKIP_ROBOT_IMPL}${SKIP_ARMPLANNING}${SKIP_MODULE}" ]]; then
	TEST_TARGET=$(go list ./...)
	if [[ -n "$SKIP_ROBOT_IMPL" ]]; then
		echo "SKIP_ROBOT_IMPL set; excluding go.viam.com/rdk/robot/impl"
		TEST_TARGET=$(echo "$TEST_TARGET" | grep -v '^go.viam.com/rdk/robot/impl$')
	fi
	if [[ -n "$SKIP_ARMPLANNING" ]]; then
		echo "SKIP_ARMPLANNING set; excluding go.viam.com/rdk/motionplan/armplanning"
		TEST_TARGET=$(echo "$TEST_TARGET" | grep -v '^go.viam.com/rdk/motionplan/armplanning$')
	fi
	if [[ -n "$SKIP_MODULE" ]]; then
		echo "SKIP_MODULE set; excluding module, module/modmanager, and example module tests"
		TEST_TARGET=$(echo "$TEST_TARGET" | grep -vE '^go\.viam\.com/rdk/(module|module/modmanager|examples/customresources/demos/.*/moduletest)$')
	fi
fi
TEST_TARGET=${TEST_TARGET:-./...}

# GO_BUILD_TAGS_EXTRA (comma- or space-separated) appends extra build tags, the
# same knob the Makefile uses for server builds. CI sets it to
# viam_rdk_cgo_have_cxx20_rt so the trajex-backed arm-streaming code and its
# tagged tests compile and run on platforms whose toolchain supports it.
GO_TAGS=no_skip
if [[ -n "$GO_BUILD_TAGS_EXTRA" ]]; then
	GO_TAGS="$GO_TAGS,$(echo $GO_BUILD_TAGS_EXTRA | tr ' ' ',')"
fi

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
PION_LOG_WARN=webrtc,datachannel,sctp gotestsum --format $FORMAT $LOGFILE -- -tags=$GO_TAGS -timeout 40m $RACE $COVER $TEST_TARGET
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
