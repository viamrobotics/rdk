//go:build !no_cgo || android

package web

import "go.viam.com/rdk/gostream"

// options configures a web service.
type options struct {
	// streamConfig is used to enable audio/video streaming over WebRTC.
	streamConfig *gostream.StreamConfig
}

// WithStreamConfig returns an Option which sets the streamConfig
// used to enable audio/video streaming over WebRTC.
func WithStreamConfig(config gostream.StreamConfig) Option {
	return newFuncOption(func(o *options) {
		o.streamConfig = &config
	})
}
