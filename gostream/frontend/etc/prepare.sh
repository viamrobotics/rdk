#!/bin/bash

rm -rf src/gen

# Ours
mkdir -p src/gen/proto
cp -R ../dist/js/proto/stream src/gen/proto

# Third-Party
