// Package fake provides a fake implementation of the worldstatestore.Service interface.
package fakePointCloud

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pc "go.viam.com/rdk/pointcloud"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

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

	pointCloudGenerator *PointCloudGenerator
}

var (
	pointcloudUUID = "pointcloud-001"
)

func init() {
	resource.RegisterService(
		worldstatestore.API,
		resource.DefaultModelFamily.WithModel("fake_pcd"),
		resource.Registration[worldstatestore.Service, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (worldstatestore.Service, error) {
			return newFakeWorldStateStore(conf.ResourceName(), logger), nil
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

func newFakeWorldStateStore(name resource.Name, logger logging.Logger) worldstatestore.Service {
	ctx, cancel := context.WithCancel(context.Background())

	fake := &WorldStateStore{
		Named:                   name.AsNamed(),
		TriviallyReconfigurable: resource.TriviallyReconfigurable{},
		TriviallyCloseable:      resource.TriviallyCloseable{},
		transforms:              make(map[string]*commonpb.Transform),
		fps:                     10,
		startTime:               time.Now(),
		changeChan:              make(chan worldstatestore.TransformChange, 100),
		streamCtx:               ctx,
		cancel:                  cancel,
		logger:                  logger,
	}

	fake.initializePointCloud()

	return fake
}

// initializeStaticTransforms creates the initial three transforms in the world.
func (f *WorldStateStore) initializePointCloud() {
	f.mu.Lock()
	defer f.mu.Unlock()

	pointCloud, err := f.pointCloudGenerator.GeneratePointCloud(100, 100)
	if err != nil {
		f.logger.Errorf("failed to generate point cloud: %v", err)
		return
	}

	header := buildUpdateHeader(0)
	f.transforms[pointcloudUUID] = &commonpb.Transform{
		ReferenceFrame: "static-pointcloud",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: 0, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Pointcloud{
				Pointcloud: &commonpb.PointCloud{
					PointCloud: pc.ToRawBytes(pointCloud, getStride()),
					Header:     header,
				},
			},
		},
		Uuid:     []byte(pointcloudUUID),
		Metadata: &structpb.Struct{},
	}
}
