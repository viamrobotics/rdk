GREPPED = $(shell grep -sao jetson /proc/device-tree/compatible)
ifneq ("$(strip $(GREPPED))", "")
   $(info Nvidia Jetson Detected)
   SERVER_DEB_PLATFORM = jetson
else ifneq ("$(wildcard /etc/rpi-issue)","")
   $(info Raspberry Pi Detected)
   SERVER_DEB_PLATFORM = pi
else
   SERVER_DEB_PLATFORM = generic
endif
SERVER_DEB_VER = 0.5

deb-server: buf-go server
	rm -rf etc/packaging/work/
	mkdir etc/packaging/work/
	cp -r etc/packaging/viam-server-$(SERVER_DEB_VER)/ etc/packaging/work/viam-server-$(SERVER_DEB_PLATFORM)-$(SERVER_DEB_VER)/
	install -D $(BIN_OUTPUT_PATH)/server etc/packaging/work/viam-server-$(SERVER_DEB_PLATFORM)-$(SERVER_DEB_VER)/usr/bin/viam-server
	cd etc/packaging/work/viam-server-$(SERVER_DEB_PLATFORM)-$(SERVER_DEB_VER)/ \
	&& sed -i "s/viam-server/viam-server-$(SERVER_DEB_PLATFORM)/g" debian/control debian/changelog \
	&& sed -i "s/viam-camera-servers/viam-camera-servers-$(SERVER_DEB_PLATFORM)/g" debian/control \
	&& dch --force-distribution -D viam -v $(SERVER_DEB_VER)+`date -u '+%Y%m%d%H%M'` "Auto-build from commit `git log --pretty=format:'%h' -n 1`" \
	&& dpkg-buildpackage -us -uc -b

deb-install: deb-server
	sudo dpkg -i etc/packaging/work/viam-server-$(SERVER_DEB_PLATFORM)_$(SERVER_DEB_VER)+*.deb

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

