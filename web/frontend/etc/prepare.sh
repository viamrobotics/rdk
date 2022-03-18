#!/bin/bash

rm -rf src/gen

# Ours
mkdir -p src/gen
cp -R ../../dist/js/proto src/gen

# Third-Party
mkdir -p src/gen/google
cp -R ../../dist/js/google/api src/gen/google

