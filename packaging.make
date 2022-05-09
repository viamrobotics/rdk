appimage: buf-go server
	cd etc/packaging/appimages && appimage-builder --recipe viam-server-latest-`uname -m`.yml
	cd etc/packaging/appimages && ./package_release.sh
	mkdir -p etc/packaging/appimages/deploy/
	mv etc/packaging/appimages/*.AppImage* etc/packaging/appimages/deploy/
	chmod 755 etc/packaging/appimages/deploy/*.AppImage

# AppImage packaging targets run in canon docker
appimage-multiarch: appimage-amd64 appimage-arm64

appimage-amd64: DOCKER_PLATFORM = --platform linux/amd64
appimage-amd64: DOCKER_TAG = amd64-cache
appimage-amd64:
	$(DOCKER_CMD) make appimage

appimage-arm64: DOCKER_PLATFORM = --platform linux/arm64
appimage-arm64: DOCKER_TAG = arm64-cache
appimage-arm64:
	$(DOCKER_CMD) make appimage

appimage-deploy:
	gsutil -m -h "Cache-Control: no-cache" cp etc/packaging/appimages/deploy/* gs://packages.viam.com/apps/viam-server/

