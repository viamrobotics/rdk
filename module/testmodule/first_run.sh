#!/usr/bin/env bash

if [[ -n "$VIAM_TEST_FAIL_RUN_FIRST" ]]; then
    echo "failed line 1"
    echo "failed line 2"
    echo "failed line 3"
    exit 1
fi

cat << EOF
success line 1
success line 2
success line 3
EOF

exit 0
