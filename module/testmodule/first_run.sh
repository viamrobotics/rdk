#!/usr/bin/env bash

echo_to_stderr() {
    >&2 echo $1
}

if [[ -n "$VIAM_TEST_FAIL_RUN_FIRST" ]]; then
    echo_to_stderr "erroring... 1"
    echo_to_stderr "erroring... 2"
    echo "failed!"

    sleep 1

    exit 1
fi

echo "running... 1"
echo_to_stderr "hiccup 1"
echo "running... 2"
echo "running... 3"
echo_to_stderr "hiccup 2"
echo "done!"

sleep 1

exit 0
