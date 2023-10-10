BUILD_CHANNEL?=local
# note: UNAME_M is overrideable because it is wrong in 32-bit arm container executing natively on 64-bit arm
UNAME_M ?= $(shell uname -m)
ifneq ($(shell which dpkg), "")
DPKG_ARCH ?= $(shell dpkg --print-architecture)
APPIMAGE_ARCH ?= $(shell dpkg --print-architecture)
endif

appimage-arch:
	# build appimage for a target architecture using existing aix + viam-server binaries
	cd etc/packaging/appimages && BUILD_CHANNEL=${BUILD_CHANNEL} UNAME_M=$(UNAME_M) DPKG_ARCH=$(DPKG_ARCH) appimage-builder --recipe viam-server.yml
	if [ "${RELEASE_TYPE}" = "stable" ]; then \
		cd etc/packaging/appimages; \
		BUILD_CHANNEL=stable UNAME_M=$(UNAME_M) DPKG_ARCH=$(DPKG_ARCH) appimage-builder --recipe viam-server.yml; \
	fi
	mkdir -p etc/packaging/appimages/deploy/
	mv etc/packaging/appimages/*.AppImage* etc/packaging/appimages/deploy/
	chmod 755 etc/packaging/appimages/deploy/*.AppImage

appimage: server-static
	cd etc/packaging/appimages && BUILD_CHANNEL=${BUILD_CHANNEL} appimage-builder --recipe viam-server-`uname -m`.yml
	if [ "${RELEASE_TYPE}" = "stable" ]; then \
		cd etc/packaging/appimages; \
		BUILD_CHANNEL=stable appimage-builder --recipe viam-server-`uname -m`.yml; \
	fi
	mkdir -p etc/packaging/appimages/deploy/
	mv etc/packaging/appimages/*.AppImage* etc/packaging/appimages/deploy/
	chmod 755 etc/packaging/appimages/deploy/*.AppImage

# AppImage packaging targets run in canon docker
appimage-multiarch: appimage-amd64 appimage-arm64

appimage-amd64:
	canon --arch amd64 make appimage

appimage-arm64:
	canon --arch arm64 make appimage

appimage-deploy:
	gsutil -m -h "Cache-Control: no-cache" cp etc/packaging/appimages/deploy/* gs://packages.viam.com/apps/viam-server/

static-release: server-static-compressed
	rm -rf etc/packaging/static/deploy/
	mkdir -p etc/packaging/static/deploy/
	cp $(BIN_OUTPUT_PATH)/viam-server etc/packaging/static/deploy/viam-server-${BUILD_CHANNEL}-`uname -m`
	if [ "${RELEASE_TYPE}" = "stable" ]; then \
		cp $(BIN_OUTPUT_PATH)/viam-server etc/packaging/static/deploy/viam-server-stable-`uname -m`; \
	fi
