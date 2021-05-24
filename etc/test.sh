#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"
cd $ROOT_DIR

if [[ "$1" == "cover" ]]; then
	COVER1=-coverprofile=coverage.txt
	COVER2=-coverprofile=coverage2.txt
fi

go test -tags=no_skip -race $COVER1 `go list ./... | grep -Ev "go.viam.com/core/(vision|rimage)"` &
PID1=$!
go test -tags=no_skip $COVER2 go.viam.com/core/vision/... go.viam.com/core/rimage/... &
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
fi
