$(NDK_ROOT):
	# todo: remove this once we are building .aar in CI
	# download ndk (used by server-android)
	cd etc && wget https://dl.google.com/android/repository/android-ndk-r26-linux.zip
	cd etc && unzip android-ndk-r26-linux.zip

.PHONY: server-android
server-android:
	GOOS=android GOARCH=arm64 CGO_ENABLED=1 \
		CC=$(shell realpath $(NDK_ROOT)/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android30-clang) \
		go build -v \
		-tags no_cgo \
		-o bin/viam-server-$(BUILD_CHANNEL)-android-aarch64 \
		./web/cmd/server

UNAME = $(shell uname)
ifeq ($(UNAME),Linux)
	PLATFORM_NDK_ROOT ?= $(HOME)/Android/Sdk/ndk/26.1.10909125
else
	PLATFORM_NDK_ROOT ?= $(HOME)/Android/Sdk/ndk/26.1.10909125
endif
DROID_CC ?= $(PLATFORM_NDK_ROOT)/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android28-clang
DROID_PREFIX = $(PWD)/etc/android/prefix

etc/android/prefix/%:
	TARGET_ARCH=$* etc/android/build-x264.sh

.SECONDARY: etc/android/prefix/aarch64 etc/android/prefix/x86_64

droid-rdk.%.aar: etc/android/prefix/aarch64 etc/android/prefix/x86_64
	# creates a per-platform android library that can be imported by native code
	# we clear CGO_LDFLAGS so this doesn't try (and fail) to link to linuxbrew where present
	$(eval JNI_ARCH := $(if $(filter arm64,$*),arm64-v8a,x86_64))
	$(eval CPU_ARCH := $(if $(filter arm64,$*),aarch64,x86_64))
        # checklinkname here is from https://github.com/wlynxg/anet#how-to-build-with-go-1230-or-later
	CGO_LDFLAGS= PKG_CONFIG_PATH=$(DROID_PREFIX)/$(CPU_ARCH)/lib/pkgconfig \
		gomobile bind -v -target android/$* -androidapi 28 -tags no_cgo \
                -ldflags="-checklinkname=0" \
		-o $@ ./web/cmd/droid
	rm -rf droidtmp/jni/$(JNI_ARCH)
	mkdir -p droidtmp/jni/$(JNI_ARCH)
	cp -d etc/android/prefix/$(CPU_ARCH)/lib/*.so* droidtmp/jni/$(JNI_ARCH)
	cd droidtmp && zip --symlinks -r ../$@ jni/$(JNI_ARCH)

droid-rdk.aar: droid-rdk.amd64.aar droid-rdk.arm64.aar
	# multi-platform AAR -- twice the size, but portable
	rm -rf droidtmp
	cp droid-rdk.arm64.aar $@.tmp
	unzip droid-rdk.amd64.aar -d droidtmp
	cd droidtmp && zip --symlinks -r ../$@.tmp jni
	mv $@.tmp $@

clean-droid:
	# note: this doesn't clean x264 checkout
	rm -rvf droid-rdk*.aar droid-rdk*.jar etc/android/prefix droidtmp
