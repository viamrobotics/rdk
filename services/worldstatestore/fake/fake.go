// Package fake provides a fake implementation of the worldstatestore.Service interface.
package fake

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"depth_camera",
	"lidar",
}

const defaultParentFrame = "world"

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

	broadcaster *worldstatestore.TransformChangeBroadcaster
	streamCtx   context.Context
	cancel      context.CancelFunc

	logger logging.Logger

	worldName   string
	parentFrame string
}

// Config is the configuration for a fake world state store.
type Config struct {
	// WorldName selects the simulation to run. It accepts both the demo worlds ("moving_geos",
	// "pcd_stream") and the simulated sensors ("depth_camera", "lidar").
	WorldName string `json:"worldName,omitempty"`
	// InputSensorType is a descriptive alias for selecting a simulated sensor ("depth_camera",
	// "lidar"). It takes precedence over WorldName when set.
	InputSensorType string `json:"input_sensor_type,omitempty"`
	// ParentFrame is the reference frame the simulated detections are reported in. Defaults to "world".
	ParentFrame string `json:"parent_frame,omitempty"`
}

// resolveWorldName returns the simulation to run, preferring InputSensorType over WorldName and
// falling back to the default. Unknown values fall back to the default.
func (conf *Config) resolveWorldName() string {
	for _, candidate := range []string{conf.InputSensorType, conf.WorldName} {
		if slices.Contains(worldNames, candidate) {
			return candidate
		}
	}
	return worldNames[0]
}

// Validate checks that the config attributes are valid for a fake world state store.
func (conf *Config) Validate(path string) ([]string, []string, error) {
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
	// Snapshot the current transforms and subscribe atomically: holding the read lock blocks any
	// concurrent mutation (whose emit runs only after it re-acquires the lock), so the new subscriber
	// receives the full current world as ADDED followed by every subsequent change, with none missed.
	f.mu.RLock()
	snapshot := make([]worldstatestore.TransformChange, 0, len(f.transforms))
	for _, tf := range f.transforms {
		snapshot = append(snapshot, worldstatestore.TransformChange{
			ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform:  tf,
		})
	}
	ch, unsubscribe := f.broadcaster.Subscribe(100)
	f.mu.RUnlock()

	idx := 0
	return worldstatestore.NewTransformChangeStream(func() (worldstatestore.TransformChange, error) {
		if idx < len(snapshot) {
			change := snapshot[idx]
			idx++
			return change, nil
		}
		select {
		case <-ctx.Done():
			unsubscribe()
			return worldstatestore.TransformChange{}, ctx.Err()
		case change, ok := <-ch:
			if !ok {
				unsubscribe()
				return worldstatestore.TransformChange{}, io.EOF
			}
			return change, nil
		}
	}), nil
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

	f.broadcaster.Close()
	return nil
}

func newFakeWorldStateStore(name resource.Name, conf *Config, logger logging.Logger) worldstatestore.Service {
	ctx, cancel := context.WithCancel(context.Background())
	worldName := worldNames[0]
	parentFrame := defaultParentFrame
	if conf != nil {
		worldName = conf.resolveWorldName()
		if conf.ParentFrame != "" {
			parentFrame = conf.ParentFrame
		}
	}

	fake := &WorldStateStore{
		Named:              name.AsNamed(),
		TriviallyCloseable: resource.TriviallyCloseable{},
		transforms:         make(map[string]*commonpb.Transform),
		fps:                10,
		startTime:          time.Now(),
		broadcaster:        worldstatestore.NewTransformChangeBroadcaster(),
		streamCtx:          ctx,
		cancel:             cancel,
		logger:             logger,
		worldName:          worldName,
		parentFrame:        parentFrame,
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
	case "depth_camera":
		world := DepthCameraWorld{worldStateStore: f}
		world.StartWorld()
	case "lidar":
		world := LidarWorld{worldStateStore: f}
		world.StartWorld()
	}
}

func (f *WorldStateStore) emitTransformChange(transform *commonpb.Transform, changeType pb.TransformChangeType, updatedFields []string) {
	f.broadcaster.Broadcast(worldstatestore.TransformChange{
		ChangeType:    changeType,
		Transform:     transform,
		UpdatedFields: updatedFields,
	})
}

func (f *WorldStateStore) emitTransformUpdate(partial *commonpb.Transform, updatedFields []string) {
	if partial == nil || len(partial.GetUuid()) == 0 {
		return
	}
	f.broadcaster.Broadcast(worldstatestore.TransformChange{
		ChangeType:    pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
		Transform:     partial,
		UpdatedFields: updatedFields,
	})
}
