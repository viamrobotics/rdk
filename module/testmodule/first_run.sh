#!/usr/bin/env bash

if [[ -n "$VIAM_TEST_FAIL_RUN_FIRST" ]]; then
    echo "Sorry, I've failed you."
    exit 1
fi

cat << EOF
-------------------------------------
Congratulations!

The test setup script ran successfully!

This message is obnoxiously large for
testing purposes.

Sincerely,
First Run Script
-------------------------------------
EOF

exit 0
