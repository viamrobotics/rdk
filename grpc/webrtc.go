package grpc

import "github.com/pion/webrtc/v3"

// DefaultWebRTCConfiguration is the default configuration to use.
var DefaultWebRTCConfiguration = webrtc.Configuration{
	// TODO(https://github.com/viamrobotics/core/issues/236): Add TURN
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:global.stun.twilio.com:3478?transport=udp"},
		},
	},
}
