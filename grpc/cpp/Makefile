CXX = g++
CPPFLAGS = -std=c++2a -Wall
SRCDIR = ./gen
IFLAGS = -I$(SRCDIR)
LDFLAGS = -L/usr/local/lib
GRPCFLAGS = `PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:${PKG_CONFIG_PATH} pkg-config --cflags protobuf grpc` `PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:${PKG_CONFIG_PATH} pkg-config --libs protobuf grpc++`
OTHER = -pthread -Wl,-lgrpc++_reflection -Wl,-ldl
SOURCES = $(SRCDIR)/proto/api/v1/robot.grpc.pb.cc $(SRCDIR)/proto/api/v1/robot.pb.cc
SOURCES += $(SRCDIR)/google/api/annotations.pb.cc $(SRCDIR)/google/api/httpbody.pb.cc
SOURCES += $(SRCDIR)/google/api/http.pb.cc

PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')
ifeq ($(PLATFORM),darwin)
	CPPFLAGS = -std=c++20 -Wall
	OTHER+=-framework CoreFoundation
endif

clean:
	rm -rf gen

buf:
	cd ../.. && buf generate --template ./grpc/cpp/buf.gen.yaml
	cd ../.. && buf generate --template ./grpc/cpp/buf.grpc.gen.yaml
	cd ../.. && buf generate --template ./grpc/cpp/buf.gen.yaml buf.build/googleapis/googleapis

setup:
	bash etc/setup.sh

# Note that if you have issues with the build, you may need to remove brew entries from PATH and LIBRARY_PATH
build_client: client.cpp
	$(CXX) $(CPPFLAGS) -o bin/client client.cpp $(SOURCES) $(IFLAGS) $(LDFLAGS) $(GRPCFLAGS) $(OTHER)

build_server: server.cpp
	$(CXX) $(CPPFLAGS) -o bin/server server.cpp $(SOURCES) $(IFLAGS) $(LDFLAGS) $(GRPCFLAGS) $(OTHER)

run_client: build_client
	./bin/client localhost:8080

run_server: build_server
	./bin/server localhost:8080
