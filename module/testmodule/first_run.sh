#!/usr/bin/env bash

if [[ -n "$VIAM_TEST_FAIL_RUN_FIRST" ]]; then
    >&2 echo "erroring... 1"
    >&2 echo "erroring... 2"
    echo "failed!"
    exit 1
fi

echo "running... 1"
>&2 echo "hiccup 1"
echo "running... 2"
echo "running... 3"
>&2 echo "hiccup 2"
echo "done!"


exit 0
