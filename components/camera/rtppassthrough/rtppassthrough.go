// Package rtppassthrough defines a Source of RTP packets
package rtppassthrough

import (
	"context"

	"github.com/google/uuid"
	"github.com/pion/rtp"
)

// NilSubscription is the value of a nil Subscription.
var NilSubscription = Subscription{ID: uuid.Nil}

type (
	// SubscriptionID is the ID of a Subscription.
	SubscriptionID = uuid.UUID
	// Subscription is the return value of a call to SubscribeRTP.
	Subscription struct {
		// ID is the ID of the Subscription
		ID SubscriptionID
		// The Terminated context will be cancelled when the RTP Subscription has terminated.
		// A successful call to Unsubscribe terminates the RTP Subscription with that ID
		// An RTP Subscription may also terminate for other internal to the Source
		// (IO errors, reconfiguration, etc)
		Terminated context.Context
	}
)

type (
	// PacketCallback is the signature of the SubscribeRTP callback.
	PacketCallback func(pkts []*rtp.Packet)
	// Source is a source of video codec data.
	Source interface {
		// SubscribeRTP begins a subscription to receive RTP packets.
		// When the Subscription terminates the context in the returned Subscription
		// is cancelled
		SubscribeRTP(ctx context.Context, bufferSize int, packetsCB PacketCallback) (Subscription, error)
		// Unsubscribe terminates the subscription with the provided SubscriptionID.
		Unsubscribe(ctx context.Context, id SubscriptionID) error
	}
)
