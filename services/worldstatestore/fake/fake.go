package fake

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/rdk/services/worldstatestore"
)

// FakeWorldStateStore implements the worldstatestore.Service interface
type FakeWorldStateStore struct {
	mu sync.RWMutex

	transforms map[string]*commonpb.Transform

	startTime time.Time
	closed    bool

	changeChan chan worldstatestore.TransformChange
	streamCtx  context.Context
	cancel     context.CancelFunc
}

// NewFakeWorldStateStore creates a new fake world state store service.
func NewFakeWorldStateStore() *FakeWorldStateStore {
	ctx, cancel := context.WithCancel(context.Background())

	fake := &FakeWorldStateStore{
		transforms: make(map[string]*commonpb.Transform),
		startTime:  time.Now(),
		changeChan: make(chan worldstatestore.TransformChange, 100),
		streamCtx:  ctx,
		cancel:     cancel,
	}

	fake.initializeStaticTransforms()
	go fake.animationLoop()
	go fake.dynamicBoxSequence()

	return fake
}

// initializeStaticTransforms creates the initial three transforms in the world.
func (f *FakeWorldStateStore) initializeStaticTransforms() {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create initial transforms
	boxUUID := "box-001"
	sphereUUID := "sphere-001"
	capsuleUUID := "capsule-001"

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
		Uuid: []byte(boxUUID),
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
		Uuid: []byte(sphereUUID),
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
		Uuid: []byte(capsuleUUID),
	}
}

// Close stops the fake service and cleans up resources.
func (f *FakeWorldStateStore) Close(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closed = true
	f.cancel()
	close(f.changeChan)
	return nil
}

// ListUUIDs returns all transform UUIDs currently in the store.
func (f *FakeWorldStateStore) ListUUIDs(ctx context.Context, extra map[string]any) ([][]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	uuids := make([][]byte, 0, len(f.transforms))
	for _, transform := range f.transforms {
		uuids = append(uuids, transform.Uuid)
	}

	return uuids, nil
}

// GetTransform returns the transform for the given UUID.
func (f *FakeWorldStateStore) GetTransform(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	transform, exists := f.transforms[string(uuid)]
	if !exists {
		return nil, errors.New("transform not found")
	}

	return transform, nil
}

// StreamTransformChanges returns a channel of transform changes.
func (f *FakeWorldStateStore) StreamTransformChanges(ctx context.Context, extra map[string]any) (<-chan worldstatestore.TransformChange, error) {
	return f.changeChan, nil
}

// DoCommand handles arbitrary commands (not implemented in fake).
func (f *FakeWorldStateStore) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"status": "do command not implemented",
	}, nil
}

func (f *FakeWorldStateStore) updateBoxTransform(elapsed time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Rotate about Y axis, 1 rotation every 5 seconds
	rotationSpeed := 2 * math.Pi / 5.0 // radians per second
	angle := rotationSpeed * elapsed.Seconds()

	if transform, exists := f.transforms["box-001"]; exists {
		transform.PoseInObserverFrame.Pose.Theta = angle * 180 / math.Pi
		f.emitTransformChange("box-001", pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED, []string{"pose"})
	}
}

func (f *FakeWorldStateStore) updateSphereTransform(elapsed time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Float up and down, 1 cycle every 5 seconds
	frequency := 2 * math.Pi / 5.0                        // radians per second
	height := math.Sin(frequency*elapsed.Seconds()) * 2.0 // ±2 units

	if transform, exists := f.transforms["sphere-001"]; exists {
		transform.PoseInObserverFrame.Pose.Y = height
		f.emitTransformChange("sphere-001", pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED, []string{"pose"})
	}
}

func (f *FakeWorldStateStore) updateCapsuleTransform(elapsed time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Scale cycle: 1 cycle every 5 seconds
	frequency := 2 * math.Pi / 5.0                           // radians per second
	scale := 1.0 + 0.5*math.Sin(frequency*elapsed.Seconds()) // 0.5x to 1.5x

	if transform, exists := f.transforms["capsule-001"]; exists {
		transform.PhysicalObject.GetCapsule().RadiusMm = 100 * scale
		transform.PhysicalObject.GetCapsule().LengthMm = 100 * scale
		f.emitTransformChange("capsule-001", pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED, []string{"pose"})
	}
}

func (f *FakeWorldStateStore) emitTransformChange(uuid string, changeType pb.TransformChangeType, updatedFields []string) {
	if transform, exists := f.transforms[uuid]; exists {
		transformCopy := &commonpb.Transform{
			ReferenceFrame:      transform.ReferenceFrame,
			PoseInObserverFrame: transform.PoseInObserverFrame,
			Uuid:                transform.Uuid,
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
}

func (f *FakeWorldStateStore) animationLoop() {
	ticker := time.NewTicker(100 * time.Millisecond) // 10 FPS
	defer ticker.Stop()

	for {
		select {
		case <-f.streamCtx.Done():
			return
		case <-ticker.C:
			if f.closed {
				return
			}
			f.updateTransforms()
		}
	}
}

func (f *FakeWorldStateStore) dynamicBoxSequence() {
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
				if f.closed {
					return
				}
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

func (f *FakeWorldStateStore) addDynamicBox(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	uuid := name + "-" + time.Now().Format("20060102150405")

	var xOffset float64
	switch name {
	case "box-front-box":
		xOffset = -5 - 2 // In front of the main box
	case "box-front-sphere":
		xOffset = 0 - 2 // In front of the sphere
	case "box-front-capsule":
		xOffset = 5 - 2 // In front of the capsule
	}

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

	f.transforms[uuid] = transform
	f.emitTransformChange(uuid, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED, nil)
}

func (f *FakeWorldStateStore) removeDynamicBox(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var uuidToRemove string
	for uuid := range f.transforms {
		if strings.HasPrefix(uuid, name) {
			uuidToRemove = uuid
			break
		}
	}

	if uuidToRemove == "" {
		return
	}

	transform := f.transforms[uuidToRemove]
	delete(f.transforms, uuidToRemove)
	change := worldstatestore.TransformChange{
		ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
		Transform:  transform,
	}

	select {
	case f.changeChan <- change:
	case <-f.streamCtx.Done():
	}
}

func (f *FakeWorldStateStore) updateTransforms() {
	elapsed := time.Since(f.startTime)

	f.updateBoxTransform(elapsed)
	f.updateSphereTransform(elapsed)
	f.updateCapsuleTransform(elapsed)
}
