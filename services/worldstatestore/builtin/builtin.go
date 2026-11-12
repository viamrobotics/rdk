// Package builtin implements a world state store service that keeps its transforms in memory.
//
// Transforms are added, updated and removed with the `DoAdd`, `DoUpdate` and `DoRemove`
// DoCommands. Nothing is persisted: the store starts with the transforms listed in its config, if
// any, and everything added since is lost when viam-server restarts or the resource is rebuilt.
//
// The model has to be added to a machine's config; it is not a default service. Machine-wide name
// uniqueness leaves room for only one default service, because every default service is named
// "builtin", and the motion service is it. For the same reason the store cannot be named
// "builtin". Left unnamed it is called "world_state_store".
package builtin

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// defaultSubscriberBuffer is how many changes a stream can fall behind before it is disconnected.
const defaultSubscriberBuffer = 1024

var errClosed = errors.New("world state store is closed")

func init() {
	resource.RegisterService(
		worldstatestore.API,
		resource.DefaultServiceModel,
		resource.Registration[worldstatestore.Service, *Config]{
			Constructor: func(
				ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
			) (worldstatestore.Service, error) {
				nativeConf, err := resource.NativeConfig[*Config](conf)
				if err != nil {
					return nil, err
				}
				svc := newBuiltIn(conf.ResourceName(), logger)
				if err := svc.seed(nativeConf.Transforms); err != nil {
					return nil, err
				}
				return svc, nil
			},
		},
	)
}

// NewBuiltIn returns an empty in-memory world state store.
func NewBuiltIn(name resource.Name, logger logging.Logger) (worldstatestore.Service, error) {
	return newBuiltIn(name, logger), nil
}

func newBuiltIn(name resource.Name, logger logging.Logger) *builtIn {
	return &builtIn{
		Named:            name.AsNamed(),
		logger:           logger,
		subscriberBuffer: defaultSubscriberBuffer,
		transforms:       make(map[string]*commonpb.Transform),
		subscribers:      make(map[uint64]chan worldstatestore.TransformChange),
	}
}

type builtIn struct {
	resource.Named
	logger logging.Logger

	// subscriberBuffer is fixed after construction. Tests shrink it to exercise slow subscribers.
	subscriberBuffer int

	// mu protects the members below. API calls and DoCommands arrive concurrently, and streams
	// unsubscribe from their own goroutines when their context ends.
	mu sync.Mutex
	// transforms is keyed by `string(uuid)`. A stored transform is never modified in place: an
	// update stores a new message. That lets change events share the stored message with
	// subscribers instead of copying it for each one.
	transforms  map[string]*commonpb.Transform
	subscribers map[uint64]chan worldstatestore.TransformChange
	nextSubID   uint64
	closed      bool
}

// ListUUIDs returns the uuid of every transform in the store, in no particular order.
func (svc *builtIn) ListUUIDs(ctx context.Context, extra map[string]any) ([][]byte, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	ret := make([][]byte, 0, len(svc.transforms))
	for _, transform := range svc.transforms {
		ret = append(ret, slices.Clone(transform.GetUuid()))
	}
	return ret, nil
}

// GetTransform returns a copy of the transform with the given uuid.
func (svc *builtIn) GetTransform(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	transform, ok := svc.transforms[string(uuid)]
	if !ok {
		return nil, fmt.Errorf("transform %q not found", uuid)
	}
	return proto.Clone(transform).(*commonpb.Transform), nil
}

// StreamTransformChanges returns a stream that first yields every transform currently in the
// store as an added change, and then every later change in the order it was applied.
//
// The stream ends with `io.EOF` when the service closes, or when the consumer falls more than a
// buffer's worth of changes behind. A consumer that sees its stream end should open a new one to
// get a fresh snapshot. The transforms a stream yields are shared with the store and must not be
// modified.
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
	ch := make(chan worldstatestore.TransformChange, len(svc.transforms)+svc.subscriberBuffer)
	for _, transform := range svc.transforms {
		ch <- worldstatestore.TransformChange{
			ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform:  transform,
		}
	}

	id := svc.nextSubID
	svc.nextSubID++
	svc.subscribers[id] = ch
	context.AfterFunc(ctx, func() { svc.unsubscribe(id) })

	return worldstatestore.NewTransformChangeStreamFromChannel(ctx, ch), nil
}

func (svc *builtIn) unsubscribe(id uint64) {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	svc.removeSubscriberLocked(id)
}

// removeSubscriberLocked closes and forgets a subscriber. Subscriber channels are only ever
// closed here, with `mu` held, which is what makes sending on them in `publishLocked` safe.
func (svc *builtIn) removeSubscriberLocked(id uint64) {
	ch, ok := svc.subscribers[id]
	if !ok {
		return
	}
	delete(svc.subscribers, id)
	close(ch)
}

// publishLocked sends a change to every subscriber. It runs with `mu` held so that subscribers
// see changes in the order they were applied; the sends never block.
func (svc *builtIn) publishLocked(change worldstatestore.TransformChange) {
	for id, ch := range svc.subscribers {
		select {
		case ch <- change:
		default:
			// Skipping the change would leave this subscriber permanently out of date, because
			// updates only carry the fields that changed. Ending its stream makes it reconnect
			// and start again from a snapshot.
			svc.logger.Warnw("Disconnecting a world state subscriber that fell too far behind",
				"id", id, "buffer", svc.subscriberBuffer)
			svc.removeSubscriberLocked(id)
		}
	}
}

// Close ends every open stream. It is safe to call more than once.
func (svc *builtIn) Close(ctx context.Context) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	svc.closed = true
	for id := range svc.subscribers {
		svc.removeSubscriberLocked(id)
	}
	return nil
}
