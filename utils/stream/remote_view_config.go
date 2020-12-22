package stream

import (
	"github.com/echolabsinc/robotcore/utils/log"

	"github.com/pion/webrtc/v3"
)

var DefaultRemoteViewConfig = RemoteViewConfig{
	StreamNumber: 0,
	WebRTCConfig: webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			// screen turnserver -vvvv -L 0.0.0.0 -J "mongodb://localhost" -r default -a -X "54.164.16.193/172.31.31.242"
			{
				URLs: []string{"stun:stun.erdaniels.com"},
			},
			{
				URLs:           []string{"turn:stun.erdaniels.com"},
				Username:       "username",
				Credential:     "password",
				CredentialType: webrtc.ICECredentialTypePassword,
			},
		},
	},
}

type RemoteViewConfig struct {
	StreamNumber int
	WebRTCConfig webrtc.Configuration
	Debug        bool
	Logger       log.Logger
}
