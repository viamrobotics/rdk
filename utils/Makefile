
packages = `pkg-config --cflags --libs realsense2 libhttpserver`

intelrealserver: intelrealserver.cpp
	g++ -g -std=c++17 $(packages) -o intelrealserver intelrealserver.cpp
