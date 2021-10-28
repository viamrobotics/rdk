#!/bin/sh

#
# Makefile
# 
#  Created on: 2018. 10. 15.
#      Author: erato
#
UNAME := $(shell uname)

ifeq ($(UNAME), Linux)
   special = -lpthread
endif

LIB_FILES = cameraserver.cpp

# setupmacos: macos.sh
#	./macos.sh

opencvserver: opencvserver.cpp $(LIB_FILES)
	g++ -g -std=c++17 opencvserver.cpp $(LIB_FILES) `pkg-config --cflags --libs opencv4 libhttpserver` $(special) -o opencvserver


CC				?= gcc
CXX				?= g++

LOCAL_PATH		:= $(shell pwd)
LOCAL_MODULE 	:= example
LOCAL_CFLAGS	:= -std=c++11 -Wall -O2 -DNDEBUG
LOCAL_CFLAGS	+= -Wno-strict-aliasing -Wno-unused-result

LOCAL_CINCLUDES := -I$(LOCAL_PATH)/include/CubeEye

LOCAL_LDLIBS	:= -lpthread \
				   -L$(LOCAL_PATH)/lib -lCubeEye

LOCAL_SRC_PATH 	:= $(LOCAL_PATH)/.
LOCAL_SRC_FILES	:= $(foreach dir, $(LOCAL_SRC_PATH), $(wildcard $(dir)/*.cpp))

vpath %.cpp	$(LOCAL_SRC_PATH)

LOCAL_OBJECT_PATH	:= $(LOCAL_PATH)/obj
LOCAL_BINARY_PATH	:= $(LOCAL_PATH)/bin
LOCAL_LD_LIB_PATH 	:=../lib

$(LOCAL_OBJECT_PATH)/%.o: %.cpp
	@echo "[build : $<]"
	$(CXX) -c -o $@ $(LOCAL_CDEFINES) $(LOCAL_CINCLUDES) $(LOCAL_CFLAGS) $<

LOCAL_OBJECT_FILES 	:= \
		$(patsubst %.cpp, $(LOCAL_OBJECT_PATH)/%.o, $(notdir $(LOCAL_SRC_FILES)))

define run_script_default
#!/bin/bash\
\n\n\
LD_LIBRARY_PATH=$(strip $(LOCAL_LD_LIB_PATH)) ./$(LOCAL_MODULE)
endef



run_script := $(run_script_default)

.PHONY: all clean
all: mkdir $(LOCAL_OBJECT_FILES)
	$(CXX) -o $(LOCAL_BINARY_PATH)/$(LOCAL_MODULE) $(wildcard $(LOCAL_OBJECT_PATH)/*.o) $(LOCAL_LDLIBS)
	@echo "$(run_script)" > $(LOCAL_PATH)/bin/run
	@chmod a+x $(LOCAL_PATH)/bin/run

clean:
	@if [ -d $(LOCAL_BINARY_PATH) ]; then rm -rf $(LOCAL_BINARY_PATH); fi
	@if [ -d $(LOCAL_OBJECT_PATH) ]; then rm -rf $(LOCAL_OBJECT_PATH); fi

mkdir:
	@if [ ! -d $(LOCAL_BINARY_PATH) ]; then mkdir $(LOCAL_BINARY_PATH); fi
	@if [ ! -d $(LOCAL_OBJECT_PATH) ]; then mkdir $(LOCAL_OBJECT_PATH); fi


intelrealserver: intelrealserver.cpp $(LIB_FILES)
	g++ -g -std=c++17 intelrealserver.cpp $(LIB_FILES) `pkg-config --cflags --libs realsense2 libhttpserver` $(special) -o intelrealserver

royaleserver: royaleserver.cpp $(LIB_FILES)
	g++ -g -std=c++17 royaleserver.cpp $(LIB_FILES) `pkg-config --cflags --libs libhttpserver royale` $(special) -o royaleserver
