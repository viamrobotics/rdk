# gostream

gostream is a library to simplify the streaming of images as video and audio chunks to audio to a series of WebRTC peers. The impetus for this existing was for doing simple GUI / audio/video streaming to a browser all within go with as little cgo as possible. The package will likely be refactored over time to support some more generalized use cases and as such will be in version 0 for the time being. Many parameters are hard coded and need to be configurable over time. Use at your own risk, and please file issues!

## TODO

- Support multiple codecs (e.g. Firefox macos-arm does not support h264 by default yet)
- Verify Windows Logitech StreamCam working
- Reconnect on server restart
- Check closes and frees
- Address code TODOs (including context.TODO)
- Documentation (inner func docs, package docs, example docs)
- Version 0.1.0
- Support removal of streams
- Synchronize audio with video

## With NixOS (Experimental)

`nix-shell --pure`

## Examples

* Stream current desktop: `go run gostream/cmd/stream_video/main.go`
* Stream camera: `go run gostream/cmd/stream_video/main.go -camera`
* Stream microphone: `go run gostream/cmd/stream_audio/main.go -playback`
* Stream microphone and camera: `go run gostream/cmd/stream_av/main.go -camera`
* Playback microphone from browser: `go run gostream/cmd/stream_audio/main.go -playback`
