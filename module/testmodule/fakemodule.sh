#!/bin/bash

# fakemodule is a completely fake module that repeatedly echos a mesasge. Used
# to test that modules that never respond to ready requests will be stopped.

while :
do
  echo "fakemodule is running"
  sleep 0.01
done
