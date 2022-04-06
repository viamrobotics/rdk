CXX = g++
CPPFLAGS = -std=c++2a -Wall
SRCDIR = ./gen
IFLAGS = -I$(SRCDIR)
GRPCFLAGS = `pkg-config --cflags --libs grpc protobuf grpc++`
OTHER = -pthread -Wl,-lgrpc++_reflection -Wl,-ldl
SOURCES = $(SRCDIR)/proto/api/v1/robot.grpc.pb.cc $(SRCDIR)/proto/api/v1/robot.pb.cc
SOURCES += $(SRCDIR)/proto/api/service/v1/metadata.grpc.pb.cc $(SRCDIR)/proto/api/service/v1/metadata.pb.cc
SOURCES += $(SRCDIR)/proto/api/common/v1/common.grpc.pb.cc $(SRCDIR)/proto/api/common/v1/common.pb.cc
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
	cd ../.. && PATH="${PATH}:`pwd`/bin" buf generate --template ./grpc/cpp/buf.gen.yaml
	cd ../.. && PATH="${PATH}:`pwd`/bin" buf generate --template ./grpc/cpp/buf.grpc.gen.yaml
	cd ../.. && PATH="${PATH}:`pwd`/bin" buf generate --template ./grpc/cpp/buf.gen.yaml buf.build/googleapis/googleapis

setup:
	sudo apt-get install -y protobuf-compiler-grpc libgrpc-dev libgrpc++-dev || brew install grpc openssl --quiet
	cd ../.. && GOBIN=`pwd`/bin go install github.com/bufbuild/buf/cmd/buf
	cd ../.. && ln -sf `which grpc_cpp_plugin` bin/protoc-gen-grpc-cpp
	pkg-config openssl || export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:`find \`which brew > /dev/null && brew --prefix\` -name openssl.pc | head -n1 | xargs dirname`

build_client: mkdir client.cpp
	$(CXX) $(CPPFLAGS) -o bin/client client.cpp $(SOURCES) $(IFLAGS) $(GRPCFLAGS) $(OTHER)

build_server: mkdir server.cpp
	$(CXX) $(CPPFLAGS) -o bin/server server.cpp $(SOURCES) $(IFLAGS) $(GRPCFLAGS) $(OTHER)

run_client: build_client
	./bin/client localhost:8080

run_server: build_server
	./bin/server localhost:8081

mkdir:
	@if [ ! -d bin ]; then mkdir bin; fi
