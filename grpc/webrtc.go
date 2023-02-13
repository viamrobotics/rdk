// Package grpc provides grpc utilities.
package grpc

import "github.com/pion/webrtc/v3"

// DefaultWebRTCConfiguration is the default configuration to use.
var DefaultWebRTCConfiguration = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:global.stun.twilio.com:3478"},
		},
	},
}
