package stream

import "github.com/pion/webrtc/v3"

var DefaultRemoteViewConfig = RemoteViewConfig{
	WebRTCConfig: webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	},
}

type RemoteViewConfig struct {
	WebRTCConfig webrtc.Configuration
	Debug        bool
}
