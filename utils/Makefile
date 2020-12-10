
packages = `pkg-config --cflags --libs realsense2 libhttpserver`

UNAME := $(shell uname)

ifeq ($(UNAME), Linux)
   special = -lpthread
endif


intelrealserver: intelrealserver.cpp
	g++ -g -std=c++17 intelrealserver.cpp $(packages) $(special) -o intelrealserver
