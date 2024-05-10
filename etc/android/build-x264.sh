#!/usr/bin/env bash
# build a static x264 distro for android

set -euxo pipefail

if [ $(uname) = Linux ]; then
	: "${NDK_ROOT:=$HOME/Android/Sdk/ndk/26.1.10909125}"
	HOST_OS=linux
else
	: "${NDK_ROOT:=$HOME/Library/Android/sdk/ndk/26.1.10909125}"
	HOST_OS=darwin
fi

API_LEVEL=28
: "${TARGET_ARCH:=aarch64}"
TOOLCHAIN=$NDK_ROOT/toolchains/llvm/prebuilt/$HOST_OS-x86_64
export CC=$TOOLCHAIN/bin/$TARGET_ARCH-linux-android$API_LEVEL-clang
EXTRAS=
if [ $TARGET_ARCH = aarch64 ]; then
	EXTRAS="--extra-cflags=-march=armv8-a"
fi
# CXX=$TOOLCHAIN/bin/$CC_ARCH-linux-android$API_LEVEL-clang++
# AR=$TOOLCHAIN/bin/llvm-ar
# LD=$CC
# RANLIB=$TOOLCHAIN/bin/llvm-ranlib
# STRIP=$TOOLCHAIN/bin/llvm-strip
# NM=$TOOLCHAIN/bin/llvm-nm
SYSROOT=$TOOLCHAIN/sysroot
DIRNAME=$(realpath $(dirname $0))
PREFIX=$DIRNAME/prefix/$TARGET_ARCH
X264_ROOT=$DIRNAME/x264

if [ ! -e $X264_ROOT ]; then
	echo checking out x264
	git clone https://code.videolan.org/videolan/x264.git -b stable $X264_ROOT
else
	echo using existing x264
fi

cd $X264_ROOT
# note: patchelf can also change the soname
if git apply --check ../unversion-soname.patch; then
	git apply ../unversion-soname.patch
elif git apply --reverse --check ../unversion-soname.patch; then
	echo "not applying patch, already applied"
else
	# note: we patch the soname because android build resolves the libx264.so -> libx264.so.164 symlink
	# if allowed, this will build successfully and then fail on startup.
	echo "soname patch could not be applied, bailing"
	exit 1
fi
./configure \
	--prefix=$PREFIX \
	--host=$TARGET_ARCH-linux-android \
	--cross-prefix=$TOOLCHAIN/bin/llvm- \
	--sysroot=$SYSROOT \
	$EXTRAS \
	--enable-shared \
	--disable-avs \
	--disable-swscale \
	--disable-lavf \
	--disable-ffms \
	--disable-gpac \
	--disable-lsmash \
&& make \
&& make install
