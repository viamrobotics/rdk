package rpcwebrtc

import (
	"context"
	"sync"

	"go.viam.com/core/utils"
)

// A MemoryCallQueue is an in-memory implementation of a call queue designed to be used for
// testing and single node deployments.
type MemoryCallQueue struct {
	mu         sync.Mutex
	hostQueues map[string]utils.RefCountedValue // of chan memoryCallOffer
}

// NewMemoryCallQueue returns a new, empty in-memory call queue.
func NewMemoryCallQueue() *MemoryCallQueue {
	return &MemoryCallQueue{hostQueues: map[string]utils.RefCountedValue{}}
}

// memoryCallOffer is the offer to start a call where information about the caller
// and how it wishes to speak is contained in the SDP.
type memoryCallOffer struct {
	sdp      string
	response chan<- CallAnswer
	discard  chan struct{} // used to stop a response to a call offer.
}

// SendOffer sends an offer associated with the given SDP to the given host.
func (queue *MemoryCallQueue) SendOffer(ctx context.Context, host, sdp string) (string, error) {
	hostQueue, release, err := queue.getOrMakeHostQueue(host)
	if err != nil {
		return "", err
	}
	defer release()

	response := make(chan CallAnswer)
	offer := memoryCallOffer{sdp: sdp, response: response, discard: make(chan struct{})}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case hostQueue <- offer:
		select {
		case <-ctx.Done():
			close(offer.discard)
			return "", ctx.Err()
		case answer := <-response:
			if answer.Err != nil {
				return "", answer.Err
			}
			return answer.SDP, nil
		}
	}
}

// RecvOffer receives the next offer for the given host. It should respond with an answer
// once a decision is made.
func (queue *MemoryCallQueue) RecvOffer(ctx context.Context, host string) (CallOfferResponder, error) {
	hostQueue, release, err := queue.getOrMakeHostQueue(host)
	if err != nil {
		return nil, err
	}
	defer release()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case offer := <-hostQueue:
		return &memoryCallOfferResponder{offer}, nil
	}
}

type memoryCallOfferResponder struct {
	offer memoryCallOffer
}

func (resp *memoryCallOfferResponder) SDP() string {
	return resp.offer.sdp
}

func (resp *memoryCallOfferResponder) Respond(ctx context.Context, ans CallAnswer) error {
	select {
	case resp.offer.response <- ans:
		return nil
	case <-resp.offer.discard:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (queue *MemoryCallQueue) getOrMakeHostQueue(host string) (chan memoryCallOffer, func(), error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	hostQueueRef, ok := queue.hostQueues[host]
	if !ok {
		hostQueueRef = utils.NewRefCountedValue(make(chan memoryCallOffer))
		queue.hostQueues[host] = hostQueueRef
	}

	return hostQueueRef.Ref().(chan memoryCallOffer), func() {
		queue.mu.Lock()
		defer queue.mu.Unlock()
		if hostQueueRef.Deref() {
			delete(queue.hostQueues, host)
		}
	}, nil
}
