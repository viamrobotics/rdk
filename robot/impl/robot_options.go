package robotimpl

import "go.viam.com/rdk/robot/web"

// options configures a Robot.
type options struct {
	// webOptions are used to initially configure the web service.
	webOptions []web.Option

	// revealSensitiveConfigDiffs will display config diffs - which may contain secret
	// information - in log statements
	revealSensitiveConfigDiffs bool
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

// WithWebOptions returns a Option which sets the streamConfig
// used to enable audio/video streaming over WebRTC.
func WithWebOptions(opts ...web.Option) Option {
	return newFuncOption(func(o *options) {
		o.webOptions = opts
	})
}

// WithRevealSensitiveConfigDiffs returns an Option which causes config
// diffs - which may contain sensitive information - to be displayed
// in logs.
func WithRevealSensitiveConfigDiffs() Option {
	return newFuncOption(func(o *options) {
		o.revealSensitiveConfigDiffs = true
	})
}
