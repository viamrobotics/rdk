#!/usr/bin/env bash

if [[ -n "$VIAM_TEST_FAIL_RUN_FIRST" ]]; then
    echo "failed line 1"
    echo "failed line 2"
    echo "failed line 3"
    exit 1
fi

echo "running... 1"
>&2 echo "hiccup 1"
echo "running... 2"
echo "running... 3"
>&2 echo "hiccup 2"
echo "done!"


exit 0
