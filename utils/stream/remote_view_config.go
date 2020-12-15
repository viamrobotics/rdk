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
	NegotiationConfig: NegotiationConfig{
		Method: NegotiationMethodPOST,
		Port:   5555,
	},
}

type NegotiationMethod int

const (
	NegotiationMethodPOST = NegotiationMethod(iota)
)

type NegotiationConfig struct {
	Method NegotiationMethod
	Port   int
}

type RemoteViewConfig struct {
	WebRTCConfig      webrtc.Configuration
	NegotiationConfig NegotiationConfig
	Debug             bool
}
