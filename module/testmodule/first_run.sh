#!/usr/bin/env bash

if [[ -n "$VIAM_TEST_FAIL_RUN_FIRST" ]]; then
    exit 1
fi

echo "first_run script ran successfully"

exit 0
