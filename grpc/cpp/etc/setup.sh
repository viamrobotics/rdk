#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

if which protoc-gen-grpc-cpp; then
  echo "protoc-gen-grpc-cpp installed"
  exit 0
fi

# Note that if you have issues with the build, you may need to remove brew entries from PATH and LIBRARY_PATH
export INSTALL_DIR=/usr/local
TEMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'grpctmp')
cd $TEMP_DIR
git clone --recurse-submodules -b v1.38.0 https://github.com/grpc/grpc
cd grpc
git checkout 54dc182082db941aa67c7c3f93ad858c99a16d7d

pushd third_party/protobuf
git checkout 909a0f36a10075c4b4bc70fdee2c7e32dd612a72 # 3.17.3
popd
pushd third_party/abseil-cpp
git checkout e1d388e7e74803050423d035e4374131b9b57919 # apple clang fix 
popd
mkdir -p third_party/abseil-cpp/cmake/build
pushd third_party/abseil-cpp/cmake/build
cmake -DCMAKE_INSTALL_PREFIX=$INSTALL_DIR \
      -DCMAKE_POSITION_INDEPENDENT_CODE=TRUE \
      -DCMAKE_CXX_STANDARD=20 \
      ../..
make -j
sudo make install
popd

mkdir -p cmake/build
pushd cmake/build
cmake -DgRPC_INSTALL=ON \
      -DgRPC_BUILD_TESTS=OFF \
      -DCMAKE_INSTALL_PREFIX=$INSTALL_DIR \
      ../..
make -j
sudo make install

sudo ln -s `which grpc_cpp_plugin` /usr/local/bin/protoc-gen-grpc-cpp
