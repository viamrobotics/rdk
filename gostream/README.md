# gostream

gostream is a library to simplify the streaming of images as video and audio chunks to audio to a series of WebRTC peers. The impetus for this existing was for doing simple GUI / audio/video streaming to a browser all within go with as little cgo as possible. The package will likely be refactored over time to support some more generalized use cases and as such will be in version 0 for the time being. Many parameters are hard coded and need to be configurable over time. Use at your own risk, and please file issues!

<p align="center">
  <a href="https://pkg.go.dev/gostream"><img src="https://pkg.go.dev/badge/gostream" alt="PkgGoDev"></a>
  <a href="https://goreportcard.com/report/gostream"><img src="https://goreportcard.com/badge/gostream" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache_2.0-blue" alt="License: Apache 2.0"></a>
</p>
<br>

## TODO

- Support multiple codecs (e.g. Firefox macos-arm does not support h264 by default yet)
- Verify Windows Logitech StreamCam working
- Reconnect on server restart
- Check closes and frees
- Address code TODOs (including context.TODO)
- Documentation (inner func docs, package docs, example docs)
- Version 0.1.0
- Tests (and integrate to GitHub Actions)
- Support removal of streams
- Synchronize audio with video

## With NixOS (Experimental)

`nix-shell --pure`

## Examples

* Stream current desktop: `make stream-desktop`
* Stream camera: `make stream-camera`
* Stream microphone: `make stream-microphone`
* Stream microphone and camera: `make stream-av`
* Playback microphone from browser: `make playback-microphone`

## Notes

## Building

### Prerequisites

* libvpx

Linux: `libvpx-dev`

macOS: `brew install libvpx`

* x264

Linux: `libx264-dev`

macOS: `brew install x264`

* opus

Linux: `libopus-dev libopusfile-dev`

macOS: `brew install opus opusfile`


## Development

### Linting

```
make lint
```

### Testing

```
make test
```

## Acknowledgements

If I somehow took code from somewhere without acknowledging it here or via the go.mod, please file an issue and let me know.
