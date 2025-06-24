# go.viam.com/utils

<p align="center">
  <a href="https://pkg.go.dev/go.viam.com/utils"><img src="https://pkg.go.dev/badge/go.viam.com/utils" alt="PkgGoDev"></a>
</a>
</p>


This is a set of go utilities you can use via importing `go.viam.com/utils`. 

## Examples

This library includes examples that demonstrate grpc functionality for a variety of contexts - see links for more information:
* [echo](https://github.com/viamrobotics/goutils/blob/main/rpc/examples/echo/README.md)

As a convenience, you can run the `make` recipes for these examples from the root of this repository via:
```
make example-{name}/{recipe}
```

For example, try running a simple echo server with:
```
make example-echo/run-server
```

## Windows Support

Windows 10 22H2 and up.

### Development Dependencies

* bash (from https://gitforwindows.org/ is good)

Support is not well tested yet.

### Known Issues

* rpc: ICE between local connections found via ICE mDNS appear to be flaky in the establishment phase.

## License check

See https://github.com/viamrobotics/rdk#license-check for instructions on how update our license policy.

## License 
Copyright 2021-2024 Viam Inc.

Apache 2.0 - See [LICENSE](https://github.com/viamrobotics/goutils/blob/main/LICENSE) file
