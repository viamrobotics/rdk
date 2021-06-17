package rpcwebrtc

import (
	"context"
)

// A CallQueue handles the transmission and reception of call offers. For every
// sending of an offer done, it is expected that there is someone to receive that
// offer and subsequently respond to it.
type CallQueue interface {
	// SendOffer sends an offer associated with the given SDP to the given host.
	SendOffer(ctx context.Context, host, sdp string) (string, error)

	// RecvOffer receives the next offer for the given host. It should respond with an answer
	// once a decision is made.
	RecvOffer(ctx context.Context, host string) (CallOfferResponder, error)
}

// CallOffer contains the information needed to offer to start a call.
type CallOffer interface {
	// The SDP contains information the caller wants to tell the answerer about.
	SDP() string
}

// A CallOfferResponder is used by an answerer to respond to a call offer with an
// answer.
type CallOfferResponder interface {
	CallOffer

	// Respond responds to the associated call offer with the given answer which contains
	// the SDP of the answerer or an error.
	Respond(ctx context.Context, ans CallAnswer) error
}

// CallAnswer is the response to an offer. An agreement to start the call
// will contain an SDP about how the answerer wishes to speak.
type CallAnswer struct {
	SDP string
	Err error
}
