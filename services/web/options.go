// Package web provides gRPC/REST/GUI APIs to control and monitor a robot.
package web

// Options are used for configuring the web server.
type Options struct {
	// AutoTitle turns on auto-tiling of any image sources added.
	AutoTile bool

	// Pprof turns on the pprof profiler accessible at /debug
	Pprof bool

	// Port sets the port to run the web server on.
	Port int

	// SharedDir is the location of static web assets.
	SharedDir string

	// SignalingAddress is where to listen to WebRTC call offers at.
	SignalingAddress string

	// Name is the FQDN of this host.
	Name string

	// Debug turns on various debugging features. For example, the echo gRPC
	// service is added.
	Debug bool

	// WebRTC configures whether or not to instruct all clients to prefer to
	// WebRTC connections over direct gRPC connections.
	WebRTC bool

	// TLSCertFile is used to enable secure communications on the hosted HTTP server.
	TLSCertFile string

	// TLSKeyFile is used to enable secure communications on the hosted HTTP server.
	TLSKeyFile string

	// internalSignaling indicates if an internal WebRTC signaling will be used.
	internalSignaling bool

	// secure determines if sever communicates are secured or not.
	secure bool
}

// NewOptions returns a default set of options which will have the
// web server run on port 8080.
func NewOptions() Options {
	return Options{
		AutoTile: true,
		Pprof:    false,
		Port:     8080,
	}
}
