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
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// defaultBacklogSlack is how many unread changes a stream may hold beyond the size of the store
// before it is disconnected. See `publishLocked`.
const defaultBacklogSlack = 1024

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
		Named:        name.AsNamed(),
		logger:       logger,
		backlogSlack: defaultBacklogSlack,
		transforms:   make(map[string]*commonpb.Transform),
		subscribers:  make(map[uint64]*subscriber),
	}
}

type builtIn struct {
	resource.Named
	logger logging.Logger

	// backlogSlack is fixed after construction. Tests shrink it to exercise slow subscribers.
	backlogSlack int

	// mu protects the members below and the subscribers themselves. API calls and DoCommands
	// arrive concurrently, and every open stream takes its changes, and unsubscribes when its
	// context ends, from a goroutine of its own.
	mu sync.Mutex
	// transforms is keyed by `string(uuid)`. A stored transform is never modified in place: an
	// update stores a new message. That lets change events share the stored message with
	// subscribers instead of copying it for each one.
	transforms  map[string]*commonpb.Transform
	subscribers map[uint64]*subscriber
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
