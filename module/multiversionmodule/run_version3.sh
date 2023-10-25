#!/bin/sh
cd `dirname $0`

go build -o ./version3 -ldflags "-X main.VERSION=v3" ./
exec ./version3 $@
