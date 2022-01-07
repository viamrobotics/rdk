// Package web provides gRPC/REST/GUI APIs to control and monitor a robot.
package web

import "go.viam.com/rdk/config"

// Options are used for configuring the web server.
type Options struct {
	// AutoTitle turns on auto-tiling of any image sources added.
	AutoTile bool

	// Pprof turns on the pprof profiler accessible at /debug
	Pprof bool

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

	// Network describes networking settings for the web server.
	Network config.NetworkConfig

	// Auth describes authentication and authorization settings for the web server.
	Auth config.AuthConfig

	// Managed signifies if this server is remotely managed (e.g. from some cloud service).
	Managed bool

	// secure determines if sever communicates are secured or not.
	secure bool

	// secureSignaling indicates if an WebRTC signaling will be secured by TLS.
	secureSignaling bool
}

// NewOptions returns a default set of options which will have the
// web server run on port 8080.
func NewOptions() Options {
	return Options{
		AutoTile: true,
		Pprof:    false,
		Network: config.NetworkConfig{
			BindAddress: "localhost:8080",
		},
	}
}
