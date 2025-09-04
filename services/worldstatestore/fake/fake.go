// Package fake provides a fake implementation of the worldstatestore.Service interface.
package fake

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
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
}

func init() {
	resource.RegisterService(
		worldstatestore.API,
		resource.DefaultModelFamily.WithModel("fake"),
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
) (<-chan worldstatestore.TransformChange, error) {
	return f.changeChan, nil
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

	fake.initializeStaticTransforms()
	fake.activeBackgroundWorkers.Add(1)
	go func() {
		defer fake.activeBackgroundWorkers.Done()
		fake.animationLoop()
	}()
	fake.activeBackgroundWorkers.Add(1)
	go func() {
		defer fake.activeBackgroundWorkers.Done()
		fake.dynamicBoxSequence()
	}()

	return fake
}

// initializeStaticTransforms creates the initial three transforms in the world.
func (f *WorldStateStore) initializeStaticTransforms() {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create initial transforms
	boxUUID := "box-001"
	sphereUUID := "sphere-001"
	capsuleUUID := "capsule-001"

	boxMetadata, err := structpb.NewStruct(map[string]any{
		"color": map[string]any{
			"r": 255,
			"g": 0,
			"b": 0,
		},
		"opacity": 0.3,
	})
	if err != nil {
		panic(err)
	}

	sphereMetadata, err := structpb.NewStruct(map[string]any{
		"color": map[string]any{
			"r": 0,
			"g": 0,
			"b": 255,
		},
		"opacity": 0.7,
	})
	if err != nil {
		panic(err)
	}

	capsuleMetadata, err := structpb.NewStruct(map[string]any{
		"color": map[string]any{
			"r": 0,
			"g": 255,
			"b": 0,
		},
		"opacity": 1.0,
	})
	if err != nil {
		panic(err)
	}

	f.transforms[boxUUID] = &commonpb.Transform{
		ReferenceFrame: "world",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: -5, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{
					DimsMm: &commonpb.Vector3{
						X: 100,
						Y: 100,
						Z: 100,
					},
				},
			},
		},
		Uuid:     []byte(boxUUID),
		Metadata: boxMetadata,
	}

	f.transforms[sphereUUID] = &commonpb.Transform{
		ReferenceFrame: "world",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: 0, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Sphere{
				Sphere: &commonpb.Sphere{
					RadiusMm: 100,
				},
			},
		},
		Uuid:     []byte(sphereUUID),
		Metadata: sphereMetadata,
	}

	f.transforms[capsuleUUID] = &commonpb.Transform{
		ReferenceFrame: "world",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: 5, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Capsule{
				Capsule: &commonpb.Capsule{
					RadiusMm: 100,
					LengthMm: 100,
				},
			},
		},
		Uuid:     []byte(capsuleUUID),
		Metadata: capsuleMetadata,
	}
}

func (f *WorldStateStore) updateBoxTransform(elapsed time.Duration) {
	rotationSpeed := 2 * math.Pi / 5.0 // radians per second
	angle := rotationSpeed * elapsed.Seconds()

	f.mu.Lock()
	if transform, exists := f.transforms["box-001"]; exists {
		transform.PoseInObserverFrame.Pose.Theta = angle * 180 / math.Pi
		uuid := transform.Uuid
		f.mu.Unlock()
		partial := &commonpb.Transform{
			Uuid: uuid,
			PoseInObserverFrame: &commonpb.PoseInFrame{
				Pose: &commonpb.Pose{Theta: angle * 180 / math.Pi},
			},
		}
		f.emitTransformUpdate(partial, []string{"poseInObserverFrame.pose.theta"})
		return
	}
	f.mu.Unlock()
}

func (f *WorldStateStore) updateSphereTransform(elapsed time.Duration) {
	frequency := 2 * math.Pi / 5.0                        // radians per second
	height := math.Sin(frequency*elapsed.Seconds()) * 2.0 // ±2 units

	f.mu.Lock()
	if transform, exists := f.transforms["sphere-001"]; exists {
		transform.PoseInObserverFrame.Pose.Y = height
		uuid := transform.Uuid
		f.mu.Unlock()
		partial := &commonpb.Transform{
			Uuid: uuid,
			PoseInObserverFrame: &commonpb.PoseInFrame{
				Pose: &commonpb.Pose{Y: height},
			},
		}
		f.emitTransformUpdate(partial, []string{"poseInObserverFrame.pose.y"})
		return
	}
	f.mu.Unlock()
}

func (f *WorldStateStore) updateCapsuleTransform(elapsed time.Duration) {
	frequency := 2 * math.Pi / 5.0                           // radians per second
	scale := 1.0 + 0.5*math.Sin(frequency*elapsed.Seconds()) // 0.5x to 1.5x
	r := 100 * scale
	l := 100 * scale

	f.mu.Lock()
	if transform, exists := f.transforms["capsule-001"]; exists {
		transform.PhysicalObject.GetCapsule().RadiusMm = r
		transform.PhysicalObject.GetCapsule().LengthMm = l
		uuid := transform.Uuid
		f.mu.Unlock()
		partial := &commonpb.Transform{
			Uuid: uuid,
			PhysicalObject: &commonpb.Geometry{
				GeometryType: &commonpb.Geometry_Capsule{
					Capsule: &commonpb.Capsule{RadiusMm: r, LengthMm: l},
				},
			},
		}
		f.emitTransformUpdate(partial, []string{"physicalObject.geometryType.value.radiusMm", "physicalObject.geometryType.value.lengthMm"})
		return
	}
	f.mu.Unlock()
}

func (f *WorldStateStore) emitTransformChange(uuid string, changeType pb.TransformChangeType, updatedFields []string) {
	var transformCopy *commonpb.Transform

	f.mu.RLock()
	if transform, exists := f.transforms[uuid]; exists {
		transformCopy = &commonpb.Transform{
			ReferenceFrame:      transform.ReferenceFrame,
			PoseInObserverFrame: transform.PoseInObserverFrame,
			Uuid:                transform.Uuid,
		}
	}
	f.mu.RUnlock()

	if transformCopy == nil {
		return
	}

	change := worldstatestore.TransformChange{
		ChangeType:    changeType,
		Transform:     transformCopy,
		UpdatedFields: updatedFields,
	}

	select {
	case f.changeChan <- change:
	case <-f.streamCtx.Done():
	default:
		// Channel is full, skip this update
	}
}

// emitTransformUpdate emits a change with a partial transform payload for UPDATE events.
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

func (f *WorldStateStore) animationLoop() {
	f.mu.RLock()
	curFPS := f.fps
	f.mu.RUnlock()
	if curFPS <= 0 {
		curFPS = 1
	}
	interval := time.Duration(float64(time.Second) / curFPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-f.streamCtx.Done():
			return
		case <-ticker.C:
			f.updateTransforms()
			// Reconfigure ticker if FPS changed
			f.mu.RLock()
			newFPS := f.fps
			f.mu.RUnlock()
			if newFPS != curFPS && newFPS > 0 {
				ticker.Stop()
				curFPS = newFPS
				interval = time.Duration(float64(time.Second) / curFPS)
				ticker = time.NewTicker(interval)
			}
		}
	}
}

func (f *WorldStateStore) dynamicBoxSequence() {
	sequence := []struct {
		action string
		name   string
		delay  time.Duration
	}{
		{"add", "box-front-box", 3 * time.Second},
		{"remove", "box-front-box", 0},
		{"add", "box-front-sphere", 3 * time.Second},
		{"remove", "box-front-sphere", 0},
		{"add", "box-front-capsule", 3 * time.Second},
		{"remove", "box-front-capsule", 0},
	}

	for {
		for _, step := range sequence {
			select {
			case <-f.streamCtx.Done():
				return
			default:
			}

			switch step.action {
			case "add":
				f.addDynamicBox(step.name)
			case "remove":
				f.removeDynamicBox(step.name)
			}

			if step.delay > 0 {
				select {
				case <-f.streamCtx.Done():
					return
				case <-time.After(step.delay):
				}
			}
		}
	}
}

func (f *WorldStateStore) addDynamicBox(name string) {
	var xOffset float64
	switch name {
	case "box-front-box":
		xOffset = -5 - 2 // In front of the main box
	case "box-front-sphere":
		xOffset = 0 - 2 // In front of the sphere
	case "box-front-capsule":
		xOffset = 5 - 2 // In front of the capsule
	}

	uuid := name + "-" + time.Now().Format("20060102150405")
	transform := &commonpb.Transform{
		ReferenceFrame: "world",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: xOffset, Y: 0, Z: 2, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		Uuid: []byte(uuid),
	}

	f.mu.Lock()
	f.transforms[uuid] = transform
	f.mu.Unlock()

	f.emitTransformChange(uuid, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED, nil)
}

func (f *WorldStateStore) removeDynamicBox(name string) {
	f.mu.Lock()

	var uuidToRemove string
	for uuid := range f.transforms {
		if strings.HasPrefix(uuid, name) {
			uuidToRemove = uuid
			break
		}
	}

	if uuidToRemove == "" {
		f.mu.Unlock()
		return
	}

	transform := f.transforms[uuidToRemove]
	delete(f.transforms, uuidToRemove)
	f.mu.Unlock()

	change := worldstatestore.TransformChange{
		ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
		Transform: &commonpb.Transform{
			Uuid: transform.Uuid,
		},
	}

	select {
	case f.changeChan <- change:
	case <-f.streamCtx.Done():
	default:
		// Channel is full, skip this update
	}
}

func (f *WorldStateStore) updateTransforms() {
	elapsed := time.Since(f.startTime)

	f.updateBoxTransform(elapsed)
	f.updateSphereTransform(elapsed)
	f.updateCapsuleTransform(elapsed)
}
