#!/bin/bash

# fakemodule is a completely fake module that repeatedly echos a message. Used
# to test that modules that never start listening are stopped.

while :
do
  echo "fakemodule is running"
  sleep 0.01
done
