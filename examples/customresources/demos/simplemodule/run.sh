#!/bin/sh
cd `dirname $0`

go build ./
exec ./simplemodule $@
