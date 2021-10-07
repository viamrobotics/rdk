#!/bin/bash

TEMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'grpctmp')
cd $TEMP_DIR

brew install libusb pkg-config homebrew/core/glfw3 cmake openssl libhttpserver
export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:"/opt/homebrew/opt/openssl@3/lib/pkgconfig"
git clone git@github.com:IntelRealSense/librealsense.git
cd librealsense
git checkout 0fa6c4f1a000059cbb80a09ee5efa54b281b22c0
git apply <<EOF
diff --git a/src/proc/color-formats-converter.cpp b/src/proc/color-formats-converter.cpp
index cc0146a04..a65090cd5 100644
--- a/src/proc/color-formats-converter.cpp
+++ b/src/proc/color-formats-converter.cpp
@@ -18,7 +18,7 @@
 #include <tmmintrin.h> // For SSSE3 intrinsics
 #endif
 
-#if defined (ANDROID) || (defined (__linux__) && !defined (__x86_64__))
+#if defined (ANDROID) || (defined (__linux__) && !defined (__x86_64__)) || defined(__aarch64__)
 
 bool has_avx() { return false; }
 
EOF
mkdir build && cd build
sudo xcode-select --reset
cmake .. -DBUILD_EXAMPLES=true -DBUILD_WITH_OPENMP=false -DHWM_OVER_XU=false -G Xcode
xcodebuild -target realsense2
cp -R ../include/librealsense2 /usr/local/include
cp Debug/* /usr/local/lib

cat <<EOF > /usr/local/lib/pkgconfig/realsense2.pc
prefix=/usr/local
exec_prefix=${prefix}
includedir=${prefix}/include
#TODO: libdir=${exec_prefix}/lib
libdir= ${prefix}/lib

Name:
Description: Intel(R) RealSense(tm) Cross Platform API
Version: 2.49.0
URL: https://github.com/IntelRealSense/librealsense
Requires.private: 
Libs: -L${libdir} -lrealsense2
Libs.private: 
Cflags: -I${includedir}

#TODO check -Wl -Bdynamic
#Libs: -L${libdir} -Wl,-Bdynamic -lrealsense
EOF
