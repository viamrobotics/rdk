package stream

import "github.com/pion/webrtc/v3"

var DefaultRemoteViewConfig = RemoteViewConfig{
	StreamNumber: 0,
	WebRTCConfig: webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.erdaniels.com"},
			},
		},
	},
}

type RemoteViewConfig struct {
	StreamNumber int
	WebRTCConfig webrtc.Configuration
	Debug        bool
}
