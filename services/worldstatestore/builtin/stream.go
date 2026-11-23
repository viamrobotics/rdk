package builtin

import (
	"context"
	"io"

	pb "go.viam.com/api/service/worldstatestore/v1"

	"go.viam.com/rdk/services/worldstatestore"
)

// subscriber is the store's end of one open change stream. Its members are protected by the
// store's `mu`, apart from `wake`.
type subscriber struct {
	// queue holds the changes the stream has not taken yet, oldest first. The transforms in it
	// are shared with the store, so a long queue of additions costs little beyond the store itself.
	queue []worldstatestore.TransformChange
	// closed is set when the store drops the subscriber, which ends its stream.
	closed bool

	// wake holds a token whenever the stream may have something new to act on: a change was
	// queued, or the subscriber was closed. It has room for one token, so waking never blocks and
	// wakes that arrive before the stream looks collapse into one.
	wake chan struct{}
}

func (sub *subscriber) wakeStream() {
	select {
	case sub.wake <- struct{}{}:
	default:
	}
}

// StreamTransformChanges returns a stream that first yields every transform currently in the
// store as an added change, and then every later change in the order it was applied.
//
// The stream ends with `io.EOF` when the service closes, or when its consumer falls too far behind
// to be worth catching up (see `publishLocked`). A consumer that sees its stream end should open
// a new one to start again from the store's current contents. The transforms a stream yields are
// shared with the store and must not be modified.
func (svc *builtIn) StreamTransformChanges(
	ctx context.Context, extra map[string]any,
) (*worldstatestore.TransformChangeStream, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.closed {
		return nil, errClosed
	}

	// The snapshot is queued and the subscriber registered under one hold of the mutex, so the
	// stream neither misses a change nor sees one that the snapshot already reflects.
	sub := &subscriber{
		queue: make([]worldstatestore.TransformChange, 0, len(svc.transforms)),
		wake:  make(chan struct{}, 1),
	}
	for _, transform := range svc.transforms {
		sub.queue = append(sub.queue, worldstatestore.TransformChange{
			ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform:  transform,
		})
	}

	id := svc.nextSubID
	svc.nextSubID++
	svc.subscribers[id] = sub
	context.AfterFunc(ctx, func() { svc.unsubscribe(id) })

	return worldstatestore.NewTransformChangeStream(func() (worldstatestore.TransformChange, error) {
		return svc.nextChange(ctx, sub)
	}), nil
}

// nextChange blocks until the subscriber has a change, its stream has ended, or `ctx` is done.
func (svc *builtIn) nextChange(ctx context.Context, sub *subscriber) (worldstatestore.TransformChange, error) {
	for {
		if err := ctx.Err(); err != nil {
			return worldstatestore.TransformChange{}, err
		}
		change, ok, err := svc.takeChange(sub)
		if ok || err != nil {
			return change, err
		}

		select {
		case <-ctx.Done():
		case <-sub.wake:
		}
	}
}

// takeChange removes and returns the oldest queued change. It reports false if there is none,
// and `io.EOF` if the stream has ended.
func (svc *builtIn) takeChange(sub *subscriber) (worldstatestore.TransformChange, bool, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if sub.closed {
		return worldstatestore.TransformChange{}, false, io.EOF
	}
	if len(sub.queue) == 0 {
		return worldstatestore.TransformChange{}, false, nil
	}

	change := sub.queue[0]
	sub.queue[0] = worldstatestore.TransformChange{}
	sub.queue = sub.queue[1:]
	if len(sub.queue) == 0 {
		// Drop the backing array, which after a large snapshot would otherwise be kept around.
		sub.queue = nil
	} else {
		// A stream is meant to have one reader. Should it have several, this keeps one wake from
		// being all that a queue of many changes ever produces.
		sub.wakeStream()
	}
	return change, true, nil
}

func (svc *builtIn) unsubscribe(id uint64) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	svc.removeSubscriberLocked(id)
}

// removeSubscriberLocked forgets a subscriber and ends its stream. Changes the stream had not
// taken yet are discarded: whoever reads the stream is either gone or about to start over.
func (svc *builtIn) removeSubscriberLocked(id uint64) {
	sub, ok := svc.subscribers[id]
	if !ok {
		return
	}
	delete(svc.subscribers, id)
	sub.closed = true
	sub.queue = nil
	sub.wakeStream()
}

// publishLocked queues the changes of one command for every subscriber. It runs with `mu` held so
// that subscribers see changes in the order they were applied.
//
// A subscriber is dropped if it still has more changes unread than the store has transforms, plus
// `backlogSlack`. At that point reading the store afresh is cheaper than catching up, and ending
// the stream is what makes its consumer do that. Skipping changes instead is not an option: an
// update only carries the fields that changed, so a consumer that missed one would stay wrong.
//
// The backlog is measured before the command's own changes are queued. A command is therefore
// never the reason its changes go undelivered, however many it carries, and a subscriber that
// keeps up is never dropped.
func (svc *builtIn) publishLocked(changes []worldstatestore.TransformChange) {
	if len(changes) == 0 {
		return
	}

	maxBacklog := len(svc.transforms) + svc.backlogSlack
	for id, sub := range svc.subscribers {
		if len(sub.queue) > maxBacklog {
			svc.logger.Warnw("Disconnecting a world state subscriber that fell too far behind",
				"id", id, "backlog", len(sub.queue), "maxBacklog", maxBacklog)
			svc.removeSubscriberLocked(id)
			continue
		}
		sub.queue = append(sub.queue, changes...)
		sub.wakeStream()
	}
}
