package client

import (
	"time"

	"go.viam.com/utils/rpc"
)

// robotClientOpts configure a Dial call. robotClientOpts are set by the RobotClientOption
// values passed to NewClient.
type robotClientOpts struct {
	// refreshEvery is how often to refresh the status/parts of the
	// robot. If unset, it will not be refreshed automatically.
	refreshEvery time.Duration

	// dialOptions are options using for clients dialing gRPC servers.
	dialOptions []rpc.DialOption
}

// RobotClientOption configures how we set up the connection.
// Cribbed from https://github.com/grpc/grpc-go/blob/aff571cc86e6e7e740130dbbb32a9741558db805/dialoptions.go#L41
type RobotClientOption interface {
	apply(*robotClientOpts)
}

// funcRobotClientOption wraps a function that modifies robotClientOpts into an
// implementation of the RobotClientOption interface.
type funcRobotClientOption struct {
	f func(*robotClientOpts)
}

func (fdo *funcRobotClientOption) apply(do *robotClientOpts) {
	fdo.f(do)
}

func newFuncRobotClientOption(f func(*robotClientOpts)) *funcRobotClientOption {
	return &funcRobotClientOption{
		f: f,
	}
}

// WithRefreshEvery returns a RobotClientOption for how often to refresh the status/parts of the
// robot.
func WithRefreshEvery(refreshEvery time.Duration) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.refreshEvery = refreshEvery
	})
}

// WithDialOptions returns a RobotClientOption which sets the options for making
// gRPC connections to other servers.
func WithDialOptions(opts ...rpc.DialOption) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.dialOptions = opts
	})
}
