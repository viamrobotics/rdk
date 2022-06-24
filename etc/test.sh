#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"
cd $ROOT_DIR

if [[ "$1" == "cover" ]]; then
	COVER1=-coverprofile=coverage.txt
	COVER2=-coverprofile=coverage2.txt
fi

# race isn't supported on the pi or jetson (and possibly other arm boards)
# https://github.com/golang/go/issues/29948
if [ "$(uname -m)" != "aarch64" ] || [ "$(uname)" != "Linux" ]; then
	RACE=-race
else
	# Tests take way longer on SBCs
	TIMEOUT="-timeout 40m"
fi

go test -tags=no_skip $RACE $COVER1 `go list ./... | grep -Ev "go.viam.com/rdk/(vision|rimage)"` &
PID1=$!
go test -tags=no_skip $TIMEOUT $COVER2 go.viam.com/rdk/vision/... go.viam.com/rdk/rimage/...&
PID2=$!

trap "kill -9 $PID1 $PID2" INT

FAIL=0
wait $PID1 || let "FAIL+=1"
wait $PID2 || let "FAIL+=2"

if [ "$FAIL" != "0" ]; then
	exit $FAIL
fi

if [[ "$1" == "cover" ]]; then
	sed '1d' coverage2.txt >> coverage.txt
	gocov convert coverage.txt | gocov-xml > coverage.xml
fi
