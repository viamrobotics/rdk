#!/bin/bash

rm -rf src/gen

mkdir -p src/gen

cd src/gen
mkdir tmp
cd tmp
git clone --filter=blob:none --no-checkout --depth 1 --sparse -b main https://github.com/viamrobotics/api.git
cd api
git sparse-checkout init --cone
git sparse-checkout add gen/js
git checkout
cd ../..
mkdir -p proto/api
mv tmp/api/gen/js/* proto/api
rm -rf tmp
cd ../..

cp -R ../../dist/js/proto/stream src/gen/proto
