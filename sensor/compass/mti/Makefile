OS=$(shell uname)

all: sdk swig
.PHONY: all

sdk:
	cd gen/third_party && $(MAKE)

swig:
	cd gen && swig -v -go -cgo -c++ -intgosize 64 mtigen.i

clean:
	cd gen/third_party && $(MAKE) clean
