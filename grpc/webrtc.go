package grpc

import "github.com/pion/webrtc/v3"

// DefaultWebRTCConfiguration is the default configuration to use.
var DefaultWebRTCConfiguration = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.viam.cloud"},
		},
		{
			URLs: []string{"turn:stun.viam.cloud"},
			// TODO(https://github.com/viamrobotics/core/issues/101): needs real creds so as to not be abused
			Username:       "username",
			Credential:     "password",
			CredentialType: webrtc.ICECredentialTypePassword,
		},
	},
}
