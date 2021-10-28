clean:
	ls gen/ | grep -v __init__.py | xargs -I {} rm -rf gen/{}

buf:
	export PATH=${PATH}:$(shell python3 -m site --user-base)/bin && cd ../.. && buf generate --template ./grpc/python/buf.gen.yaml
	export PATH=${PATH}:$(shell python3 -m site --user-base)/bin && cd ../.. && buf generate --template ./grpc/python/buf.gen.yaml buf.build/googleapis/googleapis

setup:
	pip3 install grpclib protobuf

run_server:
	python3 server.py localhost 8080

run_client:
	python3 client.py localhost 8080
