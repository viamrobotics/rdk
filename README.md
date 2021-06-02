# core

<p align="center">
  <a href="https://go.viam.com/pkg/go.viam.com/core/"><img src="https://pkg.go.dev/badge/go.viam.com/core" alt="PkgGoDev"></a>
  <a href="https://codecov.io/gh/viamrobotics/core"><img src="https://codecov.io/gh/viamrobotics/core/branch/master/graph/badge.svg?token=99YH0M8YOA" alt="CodeCov"></a>
</p>

* [Programs](#programs)
* [Dependencies](#dependencies)
* [Development](#development)

## Programs
* [lidar/cmd/view](./lidar/cmd/view) - Visualize a LIDAR device
* [rimage/cmd/both](./rimage/cmd/both) - Read color/depth data and write to an overlayed image file
* [rimage/cmd/depth](./rimage/cmd/depth) - Read depth (or color/depth) data and write pretty version to a file
* [rimage/cmd/stream_camera](./rimage/cmd/stream_camera) - Stream a local camera
* [web/cmd/server](./web/cmd/server) - Run a robot server
* [rpc/examples/echo/server](./rpc/examples/echo/server) - Run a gRPC echo example server
* [rpc/examples/echo/webrtcclient](./rpc/examples/echo/webrtcclient) - Run a gRPC echo example client over WebRTC
* [sensor/compass/cmd/client](./sensor/compass/cmd/client) - Run a general WebSocket compass
* [sensor/compass/gy511/cmd/client](./sensor/compass/gy511/cmd/client) - Run a GY511 compass
* [sensor/compass/lidar/cmd/client](./sensor/compass/lidar/cmd/client) - Run a LIDAR based compass
* [slam/cmd/server](./slam/cmd/server) - Run a SLAM implementation

### Bespoke
* [samples/boat1](./samples/boat1) - boat1 work in progress
* [samples/chess](./samples/chess) - Play chess!
* [samples/gripper1](./samples/gripper1) - gripper1 work in progress
* [samples/vision](./samples/vision) - Utilities for working with images to test out vision library code

## Dependencies

* [go1.16](https://golang.org/dl/)
* Run `make setup`

### libvpx linux source build
If libvpx is not available on your distro, run the following:

1. `git clone git@github.com:webmproject/libvpx.git`
1. `cd libvpx`
1. `mkdir build; cd build`
1. `../configure --enable-runtime-cpu-detect --enable-vp8 --enable-postproc --enable-multi-res-encoding --enable-webm-io --enable-better-hw-compatibility --enable-onthefly-bitpacking --enable-pic`
1. `sudo make install`

## Development

### Conventions
1. Always run `make lint` and test before pushing.
1. Write tests!
1. Usually merge and squash your PRs and more rarely do merge commits with each commit being a logical unit of work.
1. If you add a new package, please add it to this README.
1. If you add a new sample or command, please add it to this README.
1. Experiments should go in samples or any subdirectory with /samples/ in it. As "good" pieces get abstracted, put into a real package command directory.
1. Use imperative mood for commits (see [Git Documenation](https://git.kernel.org/pub/scm/git/git.git/tree/Documentation/SubmittingPatches?id=a5828ae6b52137b913b978e16cd2334482eb4c1f#n136)).
1. Try to avoid large merges unless you're really doing a big merge. Try to rebase (e.g. `git pull --rebase`).
1. Delete any non-release branches ASAP when done, or use a personal fork
1. Prefer metric SI prefixes where possible (e.g. millis) https://www.nist.gov/pml/weights-and-measures/metric-si-prefixes. The type of measurement (e.g. meters) is not necessary if it is implied (e.g. rulerLengthMillis).

### Protocol Buffers/gRPC

For API intercommunication, we use Protocol Buffers to serialize data and gRPC to communicate it. For more information on both technologies, see https://developers.google.com/protocol-buffers and https://grpc.io/.

Some guidelines on using these:
1. Follow the [Protobuf style guide](https://docs.buf.build/style-guide/).
1. After making changes to a `.proto` file, make sure to run `make buf` to generate new files. Make sure `protoc-gen-go-grpc` and `protoc-gen-go`, usually located in `~/go/bin`, are in your `$PATH`.
1. See [rpc/examples/echo](./rpc/examples/echo) for example usage.

### Testing with big data

Let's assume big data is > 10KiB. This kind of data is annoying to slow to pull down with git and is typically not needed except for certain tests. In order to add large data test artifacts, you need to do the following:

```
# get ARTIFACT_GOOGLE_APPLICATION_CREDENTIALS by talking to Eliot or Eric
go install go.viam.com/core/artifact/cmd/artifact
# place new artifacts in artifact_data
artifact push
git add .artifact
# commit the file at some point
```

### Testing from Github Actions

1. First make sure you have docker installed (https://docs.docker.com/get-docker/)
1. Install `act` with `brew install act`
1. Add `GIT_ACCESS_TOKEN` which is your GitHub Personal Access Token (repo scope) it to your .secrets file in the repo (see https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token and https://github.com/nektos/act#configuration)
1. Then just run `act`
