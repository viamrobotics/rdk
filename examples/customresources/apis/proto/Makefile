.PHONY: buf

default: buf

buf:
	cd ../../../../ && make tool-install
	PATH="$(PATH):$(shell pwd)/../../../../bin/gotools/$(shell uname -s)-$(shell uname -m)" buf generate
