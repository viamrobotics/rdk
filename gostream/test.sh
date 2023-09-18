#!/bin/bash
args=$(go list -f '{{.Dir}}' ./... | grep -v mmal)
set -euo pipefail
go test -tags=no_skip -race $args -json -v 2>&1 | gotestfmt
