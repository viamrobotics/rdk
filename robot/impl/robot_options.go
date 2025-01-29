package robotimpl

import (
	"go.viam.com/rdk/robot/web"
)

// options configures a Robot.
type options struct {
	// webOptions are used to initially configure the web service.
	webOptions []web.Option

	// viamHomeDir is used to configure the Viam home directory.
	viamHomeDir string

	// revealSensitiveConfigDiffs will display config diffs - which may contain secret
	// information - in log statements
	revealSensitiveConfigDiffs bool

	// shutdownCallback provides a callback for the robot to be able to shut itself down.
	shutdownCallback func()

	// whether or not to run FTDC
	enableFTDC bool

	// disableCompleteConfigWorker starts the robot without the complete config worker - should only be used for tests.
	disableCompleteConfigWorker bool
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

// WithFTDC enables FTDC.
func WithFTDC() Option {
	return newFuncOption(func(o *options) {
		o.enableFTDC = true
	})
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

// WithViamHomeDir returns a Option which sets the Viam home directory.
func WithViamHomeDir(homeDir string) Option {
	return newFuncOption(func(o *options) {
		o.viamHomeDir = homeDir
	})
}

// WithShutdownCallback returns a Option which provides a callback for the
// robot to be able to shut itself down.
func WithShutdownCallback(shutdownFunc func()) Option {
	return newFuncOption(func(o *options) {
		o.shutdownCallback = shutdownFunc
	})
}

// withDisableCompleteConfigWorker returns an Option which disables the complete config worker.
func withDisableCompleteConfigWorker() Option {
	return newFuncOption(func(o *options) {
		o.disableCompleteConfigWorker = true
	})
}
