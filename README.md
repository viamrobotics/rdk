# robotcore

## Packages

* arm - robot arms
* gripper - robot grippers
* vision - random vision utilities
  * chess - chess specific vision tools
* ml - assorted machine learning utility code
* utils - random math functions and likely other small things that don't belong elsewhere
  * intel_real_server/intelrealserver.cpp - webserver for capturing data from intel real sense cameras, then server via http, both depth and rgb
* robot - robot configuration and initalization

## Programs
* [lidar/view](./lidar/cmd/view) - Visualize a LIDAR device
* [rimage/both](./rimage/cmd/both) - Read color/depth data and write to an overlayed image file
* [rimage/depth](./rimage/cmd/depth) - Read depth (or color/depth) data and write pretty version to a file
* [rimage/stream_camera](./rimage/cmd/stream_camera) - Stream a local camera
* [robot/server](./robot/cmd/server) - Run a robot server
* [robots/hellorobot/server](./robots/hellorobot/cmd/server) - Control a hello robot
* [sensor/compass/client](./sensor/compass/cmd/client) - Run a general WebSocket compass
* [sensor/compass/gy511/client](./sensor/compass/gy511/cmd/client) - Run a GY511 compass
* [sensor/compass/lidar/client](./sensor/compass/lidar/cmd/client) - Run a LIDAR based compass
* [slam/server](./slam/cmd/server) - Run a SLAM implementation

### Bespoke
* [samples/boat1](./samples/boat1) - boat1 work in progress
* [samples/chess](./samples/chess) - Play chess!
* [samples/gripper1](./samples/gripper1) - gripper1 work in progress
* [samples/vision](./samples/vision) - Utilities for working with images to test out vision library code

## Dependencies

Make sure the following is in your shell configuration:
```
export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*
```

Also run `git config --global url.ssh://git@github.com/.insteadOf https://github.com/`


* go1.16.*
* libx264-dev (macOS: x264)
* libvpx-dev (macOS: libvpx)
	* `git clone git@github.com:webmproject/libvpx.git`
	* `cd libvpx`
	* `mkdir build; cd build`
	* `../configure --enable-runtime-cpu-detect --enable-vp8 --enable-postproc --enable-multi-res-encoding --enable-webm-io --enable-better-hw-compatibility --enable-onthefly-bitpacking --enable-pic`
	* `sudo make install`
* python2.7-dev (macOS: not needed)

### Setup

Some setup can be performed with `make setup`

### Third Party Libraries

Make sure the following is in your shell rc/profile. This will ensure any installed third party libraries will be properly found
```
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:/usr/lib/pkgconfig:$PKG_CONFIG_PATH
export LD_LIBRARY_PATH=/usr/local/lib:/usr/lib:$LD_LIBRARY_PATH
```

## Developing

### Conventions
1. Experiments should go in samples or any subdirectory with /samples/ in it. As "good" pieces get abstracted, put into a real directory.
2. Always run `make format`, `make lint`, and test before pushing.
3. Try to avoid large merges unless you're really doing a big merge. Try to rebase.
4. Write tests!
5. Delete any non-release branches ASAP when done, or use a personal fork
6. Prefer metric SI prefixes where possible (e.g. millis) https://www.nist.gov/pml/weights-and-measures/metric-si-prefixes. The type of measurement (e.g. meters) is not necessary if it is implied (e.g. rulerLengthMillis).
7. If you add a new package, please add it to this README.
8. If you add a new sample or command, please add it to this README.

### Linting

```
make lint
```

### Testing from Github Actions

1. First make sure you have docker installed (https://docs.docker.com/get-docker/)
2. Install `act` with `brew install act`
4. Add `GIT_ACCESS_TOKEN` which is your GitHub Personal Access Token (repo scope) it to your .secrets file in the repo (see https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token and https://github.com/nektos/act#configuration) 
5. Then just run `act`
