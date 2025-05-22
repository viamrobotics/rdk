package rpc

import (
	"context"
	"time"

	webrtcpb "go.viam.com/utils/proto/rpc/webrtc/v1"
)

// A WebRTCConfigProvider returns time bound WebRTC configurations.
type WebRTCConfigProvider interface {
	Config(ctx context.Context) (WebRTCConfig, error)
}

// A WebRTCConfig represents a time bound WebRTC configuration.
type WebRTCConfig struct {
	ICEServers []*webrtcpb.ICEServer
	Expires    time.Time
}
