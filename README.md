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
* chess - play chess!
* saveImageFromWebcam - really just to test out webcam capture code
* vision - utilities for working with images to test out vision library code
* robotwww - runs the web console for any robot with a config file

## Dependencies

* go1.15.*
* opencv4
* libvpx
* python2.7-dev
* swig
* yasm

### Third Party Libraries

Make sure the following is in your shell rc/profile. This will ensure any installed third party libraries will be properly found
```
export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:/usr/lib/pkgconfig
export LD_LIBRARY_PATH=/usr/local/lib:/usr/lib
```

### Python (macos)

```
make python-macos
```

### libvpx (linux)

```
git clone https://github.com/webmproject/libvpx
cd libvpx
cd build
../configure --enable-runtime-cpu-detect --enable-vp9 --enable-vp8    --enable-postproc --enable-vp9-postproc --enable-multi-res-encoding --enable-webm-io --enable-better-hw-compatibility --enable-vp9-highbitdepth --enable-onthefly-bitpacking
make -j$(nproc)
sudo make install
```

Add to your shell rc `export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:/usr/lib/pkgconfig`

## Building

```
make build
```

## Linting

```
make lint
```

## Testing from Github Actions

1. First make sure you have docker installed (https://docs.docker.com/get-docker/)
2. Install `act` with `brew install act`
3. Then just run `act`

## Some Rules
1. Experiments should go in samples or any subdirectory with /samples/ in it. As "good" pieces get abstracted, put into a real directory.
2. Always run `make format`, `make lint`, and test before pushing.
3. Try to avoid large merges unless you're really doing a big merge. Try to rebase.
4. Write tests!
5. Delete any non-release branches ASAP when done, or use a personal fork
