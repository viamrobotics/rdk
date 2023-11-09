#!/bin/sh
cd `dirname $0`

go build -o ./version2 -ldflags "-X main.VERSION=v2" ./
exec ./version2 $@
