package client

import (
	"time"

	"go.viam.com/utils/rpc"
)

// robotClientOpts configure a Dial call. robotClientOpts are set by the RobotClientOption
// values passed to NewClient.
type robotClientOpts struct {
	// refreshEvery is how often to refresh the status/parts of the
	// robot. If <=0, it will not be refreshed automatically, if unset,
	// it will automatically refresh every 10s
	refreshEvery *time.Duration

	// checkConnectedEvery is how often to check connection to the
	// robot. If <=0, it will not be refreshed automatically, if unset,
	// it will automatically refresh every 10s
	checkConnectedEvery *time.Duration

	// reconnectEvery is how often to try reconnecting the
	// robot. If <=0, it will not be refreshed automatically, if unset,
	// it will automatically refresh every 1s
	reconnectEvery *time.Duration

	// dialOptions are options using for clients dialing gRPC servers.
	dialOptions []rpc.DialOption

	// the name of the robot.
	remoteName string

	// controls whether or not sessions are disabled.
	disableSessions bool

	// enables collection of network statistics
	withNetworkStats bool

	// initialConnectionAttempts indicates the number of times to try dialing when making
	// initial connection to a machine. Defaults to three. If set to zero or a negative
	// value, will attempt to connect forever.
	initialConnectionAttempts *int

	modName string

	// skipInitialRefresh skips the resource refresh New otherwise requires before returning.
	skipInitialRefresh bool

	// doNotWaitForRunning allows connecting to still-initializing machines
	// without waiting for it to reach the running state. Note that robot clients
	// in production (not in a testing environment) will already allow connecting
	// to still-initializing machines.
	doNotWaitForRunning bool

	// see WithoutRPCSubtypes.
	withoutRPCSubtypes bool

	// overrides defaultResourcesTimeout. See WithResourcesTimeout.
	resourcesTimeout *time.Duration
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

// WithModName attaches a unary interceptor that attaches the module name for each outgoing gRPC
// request. Should only be used in Viam module library code.
func WithModName(modName string) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.modName = modName
	})
}

// WithRefreshEvery returns a RobotClientOption for how often to refresh the status/parts of the
// robot.
func WithRefreshEvery(refreshEvery time.Duration) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.refreshEvery = &refreshEvery
	})
}

// WithInitialDialAttempts sets the number of times to attempt to connect to a robot when
// initially dialing. Defaults to 3 attempts. If set to zero or a negative value, will
// attempt to connect forever.
func WithInitialDialAttempts(attempts int) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.initialConnectionAttempts = &attempts
	})
}

// WithCheckConnectedEvery returns a RobotClientOption for how often to check connection to the robot.
func WithCheckConnectedEvery(checkConnectedEvery time.Duration) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.checkConnectedEvery = &checkConnectedEvery
	})
}

// WithReconnectEvery returns a RobotClientOption for how often to reconnect the robot.
func WithReconnectEvery(reconnectEvery time.Duration) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.reconnectEvery = &reconnectEvery
	})
}

// WithRemoteName returns a RobotClientOption setting the name of the remote robot.
func WithRemoteName(remoteName string) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.remoteName = remoteName
	})
}

// WithDisableSessions returns a RobotClientOption that disables session support.
func WithDisableSessions() RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.disableSessions = true
	})
}

// WithDialOptions returns a RobotClientOption which sets the options for making
// gRPC connections to other servers.
func WithDialOptions(opts ...rpc.DialOption) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.dialOptions = opts
	})
}

// WithDoNotWaitForRunning returns a RobotClientOption that allows connecting to still-initializing machines
// without waiting for it to reach the running state. Note that robot clients
// in production (not in a testing environment) will already allow connecting
// to still-initializing machines.
func WithDoNotWaitForRunning() RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.doNotWaitForRunning = true
	})
}

// WithoutInitialRefresh returns a RobotClientOption that skips the resource refresh New
// otherwise has to complete before returning. Resources stay unknown until a periodic refresh
// succeeds, so this only suits callers that talk to the robot service directly, such as tunneling.
func WithoutInitialRefresh() RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.skipInitialRefresh = true
	})
}

// WithNetworkStats returns a RobotClientOption which sets the options for
// reporting network statistics.
func WithNetworkStats() RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.withNetworkStats = true
	})
}

// WithoutRPCSubtypes returns a RobotClientOption which skips ResourceRPCSubtypes and the
// gRPC reflection lookups resolving its descriptors. Those dominate the cost of connecting
// -- a round trip per API, against a cache that is cold on every new client -- and nothing
// needed to call a resource depends on them, since createClient builds clients from the
// compiled-in registry.
//
// ResourceRPCAPIs then returns nil, so this must not be used by viam-server connecting to a
// remote, nor by modules: both invoke APIs that are not compiled in and need the
// descriptors. It suits clients that only use compiled-in APIs, such as the CLI.
func WithoutRPCSubtypes() RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.withoutRPCSubtypes = true
	})
}

// WithResourcesTimeout returns a RobotClientOption overriding how long each call made while
// listing resources may take. Ignored if not positive, and only applied when the caller's
// context carries no deadline of its own. Raise it for machines with many resources, a slow
// link, or both.
func WithResourcesTimeout(timeout time.Duration) RobotClientOption {
	return newFuncRobotClientOption(func(o *robotClientOpts) {
		o.resourcesTimeout = &timeout
	})
}

// ExtractDialOptions extracts RPC dial options from the given options, if any exist.
func ExtractDialOptions(opts ...RobotClientOption) []rpc.DialOption {
	var rOpts robotClientOpts
	for _, opt := range opts {
		opt.apply(&rOpts)
	}
	return rOpts.dialOptions
}
