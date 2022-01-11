# gRPC C++

## Dependencies

* C++20
	* Note: Apple clang version 12.0.5 appears to have issues in `make setup`
* RDK [Dependencies](../../README.md#dependencies)
* bazel
* macOS
	* `brew install openssl`
	* `export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:/opt/homebrew/opt/openssl/lib/pkgconfig`
* Run `make setup buf`

### Server
* Run `make run_server`

### Client
* Run `make run_client`
