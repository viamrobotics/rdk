VERSION=v1.12.0

all: sdk swig
.PHONY: all

sdk:
	cd gen/third_party/rplidar_sdk-release-${VERSION}/sdk && make
	sudo cp gen/third_party/rplidar_sdk-release-${VERSION}/sdk/output/Linux/Release/librplidar_sdk.a /usr/local/bin/

swig:
	cd gen && swig -v -go -cgo -c++ -intgosize 64 rplidar.i
