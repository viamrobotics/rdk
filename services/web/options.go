// Package web provides gRPC/REST/GUI APIs to control and monitor a robot.
package web

import "go.viam.com/core/config"

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
		Network: config.NetworkConfig{
			BindAddress: "localhost:8080",
		},
	}
}
