// Package fake provides a fake implementation of the worldstatestore.Service interface.
package fake

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

var worldNames = []string{
	"moving_geos",
	"pcd_stream",
}

// WorldStateStore implements the worldstatestore.Service interface.
type WorldStateStore struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	mu sync.RWMutex

	transforms map[string]*commonpb.Transform
	fps        float64

	startTime               time.Time
	activeBackgroundWorkers sync.WaitGroup

	changeChan chan worldstatestore.TransformChange
	streamCtx  context.Context
	cancel     context.CancelFunc

	logger logging.Logger

	worldName string
}

// Config is the configuration for a fake world state store.
type Config struct {
	WorldName string `json:"worldName,omitempty"`
}

// Validate checks that the config attributes are valid for a fake world state store.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	if conf.WorldName == "" || !slices.Contains(worldNames, conf.WorldName) {
		conf.WorldName = worldNames[0]
	}
	return nil, nil, nil
}

func init() {
	resource.RegisterService(
		worldstatestore.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[worldstatestore.Service, *Config]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (worldstatestore.Service, error) {
			newConf, err := resource.NativeConfig[*Config](conf)
			if err != nil {
				return nil, err
			}
			logger.Infof("new fake world state store with name: %s", newConf.WorldName)
			return newFakeWorldStateStore(conf.ResourceName(), newConf, logger), nil
		}})
}

// BuiltInReconfigure reconfigures the fake world state store.
func (f *WorldStateStore) BuiltInReconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	// Cancel existing background workers and wait for them to stop
	f.mu.Lock()
	f.cancel()
	f.mu.Unlock()

	f.activeBackgroundWorkers.Wait()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// Use context.Background() for background workers, not the passed ctx which may be short-lived
	newCtx, cancel := context.WithCancel(context.Background())

	f.mu.Lock()
	f.transforms = make(map[string]*commonpb.Transform)
	f.fps = 10
	f.startTime = time.Now()
	f.changeChan = make(chan worldstatestore.TransformChange, 100)
	f.streamCtx = newCtx
	f.cancel = cancel
	f.worldName = newConf.WorldName
	f.mu.Unlock()

	f.logger.Infof("reconfiguring fake world state store with name: %s", newConf.WorldName)

	// startWorld acquires its own locks internally, so we must not hold the lock here
	f.startWorld()

	return nil
}

// ListUUIDs returns all transform UUIDs currently in the store.
func (f *WorldStateStore) ListUUIDs(ctx context.Context, extra map[string]any) ([][]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	uuids := make([][]byte, 0, len(f.transforms))
	for _, transform := range f.transforms {
		uuids = append(uuids, transform.Uuid)
	}

	return uuids, nil
}

// GetTransform returns the transform for the given UUID.
func (f *WorldStateStore) GetTransform(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	transform, exists := f.transforms[string(uuid)]
	if !exists {
		return nil, errors.New("transform not found")
	}

	return transform, nil
}

// StreamTransformChanges returns a channel of transform changes.
func (f *WorldStateStore) StreamTransformChanges(
	ctx context.Context,
	extra map[string]any,
) (*worldstatestore.TransformChangeStream, error) {
	return worldstatestore.NewTransformChangeStreamFromChannel(ctx, f.changeChan), nil
}

// DoCommand handles arbitrary commands. Currently accepts "fps": float64 to set the animation rate.
func (f *WorldStateStore) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if fps, ok := cmd["fps"].(float64); ok {
		if fps <= 0 {
			return nil, errors.New("fps must be greater than 0")
		}
		f.mu.Lock()
		f.fps = float64(fps)
		f.mu.Unlock()
		return map[string]any{
			"status": "fps set to " + fmt.Sprintf("%.2f", fps),
		}, nil
	}

	return map[string]any{
		"status": "command not implemented",
	}, nil
}

// Close stops the fake service and cleans up resources.
func (f *WorldStateStore) Close(ctx context.Context) error {
	f.cancel()

	done := make(chan struct{})
	go func() {
		f.activeBackgroundWorkers.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		// proceed even if workers did not exit in time
	}

	close(f.changeChan)
	return nil
}

func newFakeWorldStateStore(name resource.Name, conf *Config, logger logging.Logger) worldstatestore.Service {
	ctx, cancel := context.WithCancel(context.Background())
	var worldName string
	if conf != nil {
		worldName = conf.WorldName
	} else {
		worldName = "moving_geos"
	}

	fake := &WorldStateStore{
		Named:              name.AsNamed(),
		TriviallyCloseable: resource.TriviallyCloseable{},
		transforms:         make(map[string]*commonpb.Transform),
		fps:                10,
		startTime:          time.Now(),
		changeChan:         make(chan worldstatestore.TransformChange, 100),
		streamCtx:          ctx,
		cancel:             cancel,
		logger:             logger,
		worldName:          worldName,
	}

	fake.startWorld()

	return fake
}

func (f *WorldStateStore) startWorld() {
	switch f.worldName {
	case "moving_geos":
		world := MovingGeosWorld{
			worldStateStore: f,
		}
		world.StartWorld()
	case "pcd_stream":
		world := PointCloudWorld{
			worldStateStore: f,
			noise:           NewPerlin(2, 2, 8, 0),
			spacing:         10,
		}
		world.StartWorld()
	}
}

func (f *WorldStateStore) emitTransformChange(transform *commonpb.Transform, changeType pb.TransformChangeType, updatedFields []string) {
	change := worldstatestore.TransformChange{
		ChangeType:    changeType,
		Transform:     transform,
		UpdatedFields: updatedFields,
	}

	select {
	case f.changeChan <- change:
	case <-f.streamCtx.Done():
	default:
		// Channel is full, skip this update
	}
}

func (f *WorldStateStore) emitTransformUpdate(partial *commonpb.Transform, updatedFields []string) {
	if partial == nil || len(partial.GetUuid()) == 0 {
		return
	}
	change := worldstatestore.TransformChange{
		ChangeType:    pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
		Transform:     partial,
		UpdatedFields: updatedFields,
	}
	select {
	case f.changeChan <- change:
	case <-f.streamCtx.Done():
	default:
		// Channel is full, skip this update
	}
}
