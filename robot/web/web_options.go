//go:build cgo
package web

import "github.com/viamrobotics/gostream"

// options configures a web service.
type options struct {
	// streamConfig is used to enable audio/video streaming over WebRTC.
	streamConfig *gostream.StreamConfig
}

// Option configures how we set up the web service.
// Cribbed from https://github.com/grpc/grpc-go/blob/aff571cc86e6e7e740130dbbb32a9741558db805/dialoptions.go#L41
type Option interface {
	apply(*options)
}

// funcOption wraps a function that modifies options into an
// implementation of the Option interface.
type funcOption struct {
	f func(*options)
}

func (fdo *funcOption) apply(do *options) {
	fdo.f(do)
}

func newFuncOption(f func(*options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

// WithStreamConfig returns an Option which sets the streamConfig
// used to enable audio/video streaming over WebRTC.
func WithStreamConfig(config gostream.StreamConfig) Option {
	return newFuncOption(func(o *options) {
		o.streamConfig = &config
	})
}
