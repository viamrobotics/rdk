package worldstatestore

import (
	"sync"

	pb "go.viam.com/api/service/worldstatestore/v1"
)

// TransformChangeBroadcaster fans out TransformChanges to multiple subscribers, each on its own channel
// (a single shared channel would deliver each change to only one consumer).
//
// Structural changes (ADDED/REMOVED) are never silently dropped: a full-buffer subscriber is
// disconnected to re-sync from a snapshot. A full buffer may drop an UPDATED; a later change supersedes it.
type TransformChangeBroadcaster struct {
	mu     sync.Mutex
	subs   map[int]*broadcastSub
	nextID int
	closed bool
}

type broadcastSub struct {
	ch        chan TransformChange
	closeOnce sync.Once
}

func (s *broadcastSub) close() {
	s.closeOnce.Do(func() { close(s.ch) })
}

// NewTransformChangeBroadcaster returns a ready-to-use broadcaster.
func NewTransformChangeBroadcaster() *TransformChangeBroadcaster {
	return &TransformChangeBroadcaster{subs: make(map[int]*broadcastSub)}
}

// Subscribe registers a subscriber, returning its channel and an idempotent unsubscribe. The channel
// closes on unsubscribe, structural overflow, or broadcaster close.
func (b *TransformChangeBroadcaster) Subscribe(bufferSize int) (<-chan TransformChange, func()) {
	if bufferSize < 1 {
		bufferSize = 1
	}
	sub := &broadcastSub{ch: make(chan TransformChange, bufferSize)}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		sub.close()
		return sub.ch, func() {}
	}
	id := b.nextID
	b.nextID++
	b.subs[id] = sub
	return sub.ch, func() { b.removeSub(id) }
}

func (b *TransformChangeBroadcaster) removeSub(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sub, ok := b.subs[id]; ok {
		delete(b.subs, id)
		sub.close()
	}
}

// Broadcast delivers a change to all current subscribers per the delivery policy described on the type.
func (b *TransformChangeBroadcaster) Broadcast(change TransformChange) {
	structural := change.ChangeType == pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED ||
		change.ChangeType == pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	for id, sub := range b.subs {
		select {
		case sub.ch <- change:
		default:
			if structural {
				// Cannot reliably deliver a structural change; disconnect so the client re-syncs.
				delete(b.subs, id)
				sub.close()
			}
			// A full buffer simply drops an UPDATED; a later change supersedes it.
		}
	}
}

// Close disconnects all subscribers and prevents future broadcasts. Idempotent.
func (b *TransformChangeBroadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, sub := range b.subs {
		delete(b.subs, id)
		sub.close()
	}
}
