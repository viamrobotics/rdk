PLATFORM=$(shell uname -s | tr '[:upper:]' '[:lower:]')

ifeq ($(PLATFORM),darwin)
	GRPC_JAVA="https://repo1.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.39.0/protoc-gen-grpc-java-1.39.0-osx-x86_64.exe"
else ifeq ($(PLATFORM),linux)
	GRPC_JAVA="https://repo1.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.39.0/protoc-gen-grpc-java-1.39.0-linux-x86_64.exe"
else
$(error unknown platform $(PLATFORM))
endif

buf:
	cd ../.. && buf generate --template ./grpc/java/buf.gen.yaml
	cd ../.. && buf generate --template ./grpc/java/buf.gen.yaml buf.build/beta/googleapis
	rm -rf src/main/gen/io
	rm -rf src/main/gen/google
	ls src/main/gen/com/google | grep -v api | xargs -I {} rm -rf src/main/gen/com/google/{}

setup:
	sudo curl -o /usr/local/bin/protoc-gen-grpc-java -L ${GRPC_JAVA}
	sudo chmod +x /usr/local/bin/protoc-gen-grpc-java

run_client:
	./gradlew runClient --args="localhost:8080"
