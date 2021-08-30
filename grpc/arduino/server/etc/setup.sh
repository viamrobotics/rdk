#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

cd $DIR/..
mkdir -p third_party
cd third_party
rm -rf arduino-grpc
git clone https://github.com/viamrobotics/arduino-grpc.git
cd arduino-grpc
git checkout 3b09c9ed47e5943cd137d66a1a95d3f6e3091776

rm -rf $DIR/../src/arduino
rm -rf $DIR/../src/grpc
rm -rf $DIR/../src/http2
rm -rf $DIR/../src/utils
ln -s $DIR/../third_party/arduino-grpc/arduino $DIR/../src
ln -s $DIR/../third_party/arduino-grpc/grpc $DIR/../src
ln -s $DIR/../third_party/arduino-grpc/http2 $DIR/../src
ln -s $DIR/../third_party/arduino-grpc/utils $DIR/../src
