# robotcore

## Packages

* arm - robot arms
* gripper - robot grippers
* vision - random vision utilities
  * chess - chess specific vision tools
* ml - assorted machine learning utility code
* rcutil - random math functions and likely other small things that don't belong elsewhere
* utils - non golang software
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
