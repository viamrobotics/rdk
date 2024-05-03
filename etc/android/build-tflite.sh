#!/bin/bash -e

if [[ -z "${ANDROID_NDK}" ]]; then
    echo "Must provide ANDROID_NDK in environment" 1>&2
    exit 1
fi

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="${DIR}/../../"
source ${ANDROID_NDK}/build/tools/ndk_bin_common.sh
NDK_TOOLCHAIN=${ANDROID_NDK}/toolchains/llvm/prebuilt/${HOST_TAG}

# ripped from private sysops repo
cd ~ && mkdir -p tensorflow/build_arm64-v8a tensorflow/build_x86_64 && cd tensorflow
curl -L https://github.com/tensorflow/tensorflow/archive/refs/tags/v2.12.0.tar.gz | tar -xz
patch -p1 -d tensorflow-2.12.0 < ${DIR}/tflite.patch
function build() {
  local arch=$1
  pushd ~/tensorflow/build_$arch
  cmake -DCMAKE_TOOLCHAIN_FILE=${ANDROID_NDK}/build/cmake/android.toolchain.cmake \
  -DANDROID_ABI=$arch ../tensorflow-2.12.0/tensorflow/lite/c
  cmake --build . -j
  ${NDK_TOOLCHAIN}/bin/llvm-strip --strip-unneeded libtensorflowlite_c.so
  local dest=${ROOT_DIR}/services/mlmodel/tflitecpu/android/jni/$arch
  mkdir -p $dest
  cp libtensorflowlite_c.so $dest
  popd
}
for arch in arm64-v8a x86_64; do
  build $arch
done
if [ $KEEP_TFLITE_SRC != "1" ]; then
  echo "cleaning up source and build"
  rm -rf ~/tensorflow/
else
  echo "cleaning up build, keeping source"
  rm -rf ~/tensorflow/build_*
fi
