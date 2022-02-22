// Package web provides gRPC/REST/GUI APIs to control and monitor a robot.
package web

import (
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
)

// Options are used for configuring the web server.
type Options struct {
	// Pprof turns on the pprof profiler accessible at /debug
	Pprof bool

	// SharedDir is the location of static web assets.
	SharedDir string

	// SignalingAddress is where to listen to WebRTC call offers at.
	SignalingAddress string

	// SignalingDialOpts are the dial options to used for signaling.
	SignalingDialOpts []rpc.DialOption

	// FQDN is the FQDN of this host. It is assumed this FQDN is unique to
	// one robot.
	FQDN string

	// LocalFQDN is the local FQDN of this host used for UI links
	// and SDK connections. It is assumed this FQDN is unique to one
	// robot.
	LocalFQDN string

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

	// baked information when managed to make local UI simpler
	BakedAuthEntity string
	BakedAuthCreds  rpc.Credentials
}

// NewOptions returns a default set of options which will have the
// web server run on config.DefaultBindAddress.
func NewOptions() Options {
	return Options{
		Pprof: false,
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				BindAddress:           config.DefaultBindAddress,
				BindAddressDefaultSet: true,
			},
		},
	}
}
