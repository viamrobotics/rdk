BUILD_CHANNEL?=local

appimage: server-static
	cd etc/packaging/appimages && BUILD_CHANNEL=${BUILD_CHANNEL} appimage-builder --recipe viam-server-`uname -m`.yml
	cd etc/packaging/appimages && ./package_release.sh
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

static-release: server-static
	rm -rf etc/packaging/static/deploy/
	mkdir -p etc/packaging/static/deploy/
	cp $(BIN_OUTPUT_PATH)/viam-server etc/packaging/static/deploy/viam-server-${BUILD_CHANNEL}-$(shell uname -m)
	cd etc/packaging/appimages && ./package_release.sh
