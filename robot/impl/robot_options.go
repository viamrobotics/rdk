package robotimpl

// options configures a Robot.
type options struct {
	// webOptions are used to initially configure the web service.

	// viamHomeDir is used to configure the Viam home directory.
	viamHomeDir string

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
