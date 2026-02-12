BUILD_CHANNEL?=local
# note: UNAME_M is overrideable because it is wrong in 32-bit arm container executing natively on 64-bit arm
UNAME_M ?= $(shell uname -m)
ifneq ($(shell which dpkg 2>/dev/null), "")
DPKG_ARCH ?= $(shell dpkg --print-architecture)
APPIMAGE_ARCH ?= $(shell dpkg --print-architecture)
endif

PRERELEASE_PATH := $(if $(findstring -dev,$(BUILD_CHANNEL)),"prerelease/","")

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

static-release: $(BIN_OUTPUT_PATH)/viam-server-static-compressed
	rm -rf etc/packaging/static/deploy/
	mkdir -p etc/packaging/static/deploy/
	cp $^ etc/packaging/static/deploy/viam-server-${BUILD_CHANNEL}-${UNAME_M}
	if [ "${RELEASE_TYPE}" = "stable" ] || [ "${RELEASE_TYPE}" = "latest" ]; then \
		cp $^ etc/packaging/static/deploy/viam-server-${RELEASE_TYPE}-${UNAME_M}; \
	fi
	rm -rf etc/packaging/static/manifest/
	mkdir -p etc/packaging/static/manifest/
	go run etc/subsystem_manifest/main.go \
		--binary-path etc/packaging/static/deploy/viam-server-${BUILD_CHANNEL}-${UNAME_M} \
		--upload-path "packages.viam.com/apps/viam-server/${PRERELEASE_PATH}viam-server-${BUILD_CHANNEL}-${UNAME_M}" \
		--version ${BUILD_CHANNEL} \
		--arch ${UNAME_M} \
		--output-path etc/packaging/static/manifest/viam-server-${BUILD_CHANNEL}-${UNAME_M}.json

static-release-win:
	rm -f bin/static/viam-server-windows.exe
	GOOS=windows GOARCH=amd64 go build -tags no_cgo,osusergo,netgo -ldflags="-extldflags=-static $(COMMON_LDFLAGS)" -o bin/static/viam-server-windows.exe ./web/cmd/server
	upx --best --lzma bin/static/viam-server-windows.exe

	rm -rf etc/packaging/static/deploy/
	mkdir -p etc/packaging/static/deploy/
	cp bin/static/viam-server-windows.exe etc/packaging/static/deploy/viam-server-${BUILD_CHANNEL}-windows-${UNAME_M}
	# note: the stable/latest file still has a .exe extension because we expect this to be a known URL that people download + want to have usable.
	if [ "${RELEASE_TYPE}" = "stable" ] || [ "${RELEASE_TYPE}" = "latest" ]; then \
		cp bin/static/viam-server-windows.exe etc/packaging/static/deploy/viam-server-${RELEASE_TYPE}-windows-${UNAME_M}.exe; \
	fi

	# note: GOOS=windows would break this on a linux runner
	go run -tags no_cgo ./web/cmd/server --dump-resources win-resources.json

	rm -rf etc/packaging/static/manifest/
	mkdir -p etc/packaging/static/manifest/
	go run ./etc/subsystem_manifest \
		--binary-path etc/packaging/static/deploy/viam-server-${BUILD_CHANNEL}-windows-${UNAME_M} \
		--upload-path packages.viam.com/apps/viam-server/${PRERELEASE_PATH}viam-server-${BUILD_CHANNEL}-windows-${UNAME_M} \
		--version ${BUILD_CHANNEL} \
		--arch ${UNAME_M} \
		--resources-json win-resources.json \
		--output-path etc/packaging/static/manifest/viam-server-${BUILD_CHANNEL}-windows-${UNAME_M}.json
