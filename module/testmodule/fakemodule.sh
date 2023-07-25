#!/bin/bash

# fakemodule is a completely fake module that echos a message and exits. Used
# to test that modules that never respond to ready requests will not be
# restarted.

echo "this is a fake module; exiting now"
