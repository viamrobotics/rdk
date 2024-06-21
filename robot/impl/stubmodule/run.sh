#!/usr/bin/env bash

cd $(dirname $0)
exec go run main.go $@
