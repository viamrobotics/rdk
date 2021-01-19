OS=$(shell uname)
VERSION=v1.12.0

all: sdk swig
.PHONY: all

sdk:
	@echo $(OS)
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && make
	sudo cp gen/third_party/rplidar_sdk-release-${VERSION}/sdk/output/${OS}/Release/librplidar_sdk.a /usr/local/lib/
	sudo chmod 755 /usr/local/lib/librplidar_sdk.a

swig:
	cd gen && swig -v -go -cgo -c++ -intgosize 64 rplidar.i
