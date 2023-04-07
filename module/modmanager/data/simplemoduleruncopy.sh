#!/bin/sh
#
# simplemoduleruncopy is a copy of examples/customresources/demos/simplemodule/run.sh
# used to test reconfiguring modules to have new executable paths.
cd `dirname $0`
cd ../../../examples/customresources/demos/simplemodule

go build ./
exec ./simplemodule $@
