// Package rtppassthrough defines a source of RTP packets
package rtppassthrough

import (
	"context"

	"github.com/pion/rtp"
)

type (
	// PacketCallback is the signature of the SubscribeRTP callback.
	PacketCallback func(pkts []*rtp.Packet) error
	// Source is a source of video codec data.
	Source interface {
		SubscribeRTP(ctx context.Context, bufferSize int, packetsCB PacketCallback) (SubscriptionID, error)
		Unsubscribe(ctx context.Context, id SubscriptionID) error
	}
)
