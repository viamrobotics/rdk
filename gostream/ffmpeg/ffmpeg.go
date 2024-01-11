//go:build cgo && !android

// Package ffmpeg is a wrapper around FFmpeg/release6.1.
// See: https://github.com/FFmpeg/FFmpeg/tree/release/6.1
package ffmpeg

//#cgo CFLAGS: -I${SRCDIR}/include
//#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/Linux-aarch64/lib -lavformat -lavcodec -lavutil -lswscale -lm
//#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/Linux-x86_64/lib -lavformat -lavcodec -lavutil -lswscale -lm
//#cgo linux,arm LDFLAGS: -L${SRCDIR}/Linux-armv7l/lib -lavformat -lavcodec -lavutil -lm
import "C"
