#!/bin/sh
cd `dirname $0`

go build -o ./version1 -ldflags "-X main.VERSION=v1" ./
exec ./version1 $@
