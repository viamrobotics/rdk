package fake

import (
	"math"
	"strings"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/rdk/services/worldstatestore"
	"google.golang.org/protobuf/types/known/structpb"
)

type MovingGeosWorld struct {
	worldStateStore *WorldStateStore
}

var (
	boxUUID     = "box-001"
	sphereUUID  = "sphere-001"
	capsuleUUID = "capsule-001"

	boxMetadata = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"color": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"r": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
							"g": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"b": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
						},
					},
				},
			},
			"opacity": {
				Kind: &structpb.Value_NumberValue{
					NumberValue: 0.5,
				},
			},
		},
	}
	sphereMetadata = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"color": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"r": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"g": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"b": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
						},
					},
				},
			},
			"opacity": {
				Kind: &structpb.Value_NumberValue{
					NumberValue: 0.7,
				},
			},
		},
	}
	capsuleMetadata = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"color": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"r": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"g": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
							"b": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
						},
					},
				},
			},
			"opacity": {
				Kind: &structpb.Value_NumberValue{
					NumberValue: 1.0,
				},
			},
		},
	}
	dynamicBoxMetadata = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"color": {
				Kind: &structpb.Value_StructValue{
					StructValue: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"r": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
							"g": {Kind: &structpb.Value_NumberValue{NumberValue: 0}},
							"b": {Kind: &structpb.Value_NumberValue{NumberValue: 255}},
						},
					},
				},
			},
			"opacity": {
				Kind: &structpb.Value_NumberValue{
					NumberValue: 0.3,
				},
			},
		},
	}
)

func (w *MovingGeosWorld) StartWorld() {
	w.initializeStaticTransforms()
	w.worldStateStore.activeBackgroundWorkers.Add(1)
	go func() {
		defer w.worldStateStore.activeBackgroundWorkers.Done()
		w.animationLoop()
	}()
	w.worldStateStore.activeBackgroundWorkers.Add(1)
	go func() {
		defer w.worldStateStore.activeBackgroundWorkers.Done()
		w.dynamicBoxSequence()
	}()
}

// initializeStaticTransforms creates the initial three transforms in the world.
func (w *MovingGeosWorld) initializeStaticTransforms() {
	w.worldStateStore.mu.Lock()
	defer w.worldStateStore.mu.Unlock()

	// Create initial transforms
	w.worldStateStore.transforms[boxUUID] = &commonpb.Transform{
		ReferenceFrame: "static-box",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: -5000, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{
					DimsMm: &commonpb.Vector3{
						X: 1000,
						Y: 1000,
						Z: 1000,
					},
				},
			},
		},
		Uuid:     []byte(boxUUID),
		Metadata: boxMetadata,
	}

	w.worldStateStore.transforms[sphereUUID] = &commonpb.Transform{
		ReferenceFrame: "static-sphere",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: 0, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Sphere{
				Sphere: &commonpb.Sphere{
					RadiusMm: 500,
				},
			},
		},
		Uuid:     []byte(sphereUUID),
		Metadata: sphereMetadata,
	}

	w.worldStateStore.transforms[capsuleUUID] = &commonpb.Transform{
		ReferenceFrame: "static-capsule",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: 5000, Y: 0, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Capsule{
				Capsule: &commonpb.Capsule{
					RadiusMm: 125,
					LengthMm: 1000,
				},
			},
		},
		Uuid:     []byte(capsuleUUID),
		Metadata: capsuleMetadata,
	}
}

func (w *MovingGeosWorld) updateBoxTransform(elapsed time.Duration) {
	rotationSpeed := 2 * math.Pi / 5.0 // radians per second
	angle := rotationSpeed * elapsed.Seconds()

	w.worldStateStore.mu.Lock()
	if transform, exists := w.worldStateStore.transforms["box-001"]; exists {
		theta := angle * 180 / math.Pi
		transform.PoseInObserverFrame.Pose.Theta = theta
		w.worldStateStore.mu.Unlock()
		w.worldStateStore.emitTransformUpdate(&commonpb.Transform{
			Uuid: transform.Uuid,
			PoseInObserverFrame: &commonpb.PoseInFrame{
				Pose: &commonpb.Pose{
					Theta: theta,
				},
			},
		}, []string{"poseInObserverFrame.pose.theta"})
		return
	}
	w.worldStateStore.mu.Unlock()
}

func (w *MovingGeosWorld) updateSphereTransform(elapsed time.Duration) {
	frequency := 2 * math.Pi / 5.0                           // radians per second
	height := math.Sin(frequency*elapsed.Seconds()) * 2000.0 // ±2 units

	w.worldStateStore.mu.Lock()
	if transform, exists := w.worldStateStore.transforms["sphere-001"]; exists {
		transform.PoseInObserverFrame.Pose.Y = height
		w.worldStateStore.mu.Unlock()
		w.worldStateStore.emitTransformUpdate(&commonpb.Transform{
			Uuid: transform.Uuid,
			PoseInObserverFrame: &commonpb.PoseInFrame{
				Pose: &commonpb.Pose{
					Y: height,
				},
			},
		}, []string{"poseInObserverFrame.pose.y"})
		return
	}
	w.worldStateStore.mu.Unlock()
}

func (w *MovingGeosWorld) updateCapsuleTransform(elapsed time.Duration) {
	frequency := 2 * math.Pi / 5.0                           // radians per second
	scale := 1.0 + 0.5*math.Sin(frequency*elapsed.Seconds()) // 0.5x to 1.5x
	r := 125 * scale
	l := 1000 * scale

	w.worldStateStore.mu.Lock()
	if transform, exists := w.worldStateStore.transforms["capsule-001"]; exists {
		transform.PhysicalObject.GetCapsule().RadiusMm = r
		transform.PhysicalObject.GetCapsule().LengthMm = l
		w.worldStateStore.mu.Unlock()
		w.worldStateStore.emitTransformUpdate(&commonpb.Transform{
			Uuid: transform.Uuid,
			PhysicalObject: &commonpb.Geometry{
				GeometryType: &commonpb.Geometry_Capsule{
					Capsule: &commonpb.Capsule{
						RadiusMm: r,
						LengthMm: l,
					},
				},
			},
		}, []string{"physicalObject.geometryType.value.radiusMm", "physicalObject.geometryType.value.lengthMm"})
		return
	}
	w.worldStateStore.mu.Unlock()
}

func (w *MovingGeosWorld) animationLoop() {
	w.worldStateStore.mu.RLock()
	curFPS := w.worldStateStore.fps
	w.worldStateStore.mu.RUnlock()
	if curFPS <= 0 {
		curFPS = 1
	}
	interval := time.Duration(float64(time.Second) / curFPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.worldStateStore.streamCtx.Done():
			return
		case <-ticker.C:
			w.updateTransforms()
			// Reconfigure ticker if FPS changed
			w.worldStateStore.mu.RLock()
			newFPS := w.worldStateStore.fps
			w.worldStateStore.mu.RUnlock()
			if newFPS != curFPS && newFPS > 0 {
				ticker.Stop()
				curFPS = newFPS
				interval = time.Duration(float64(time.Second) / curFPS)
				ticker = time.NewTicker(interval)
			}
		}
	}
}

func (w *MovingGeosWorld) dynamicBoxSequence() {
	delay := 3 * time.Second
	sequence := []struct {
		action string
		name   string
	}{
		{"add", "box-front-box"},
		{"remove", "box-front-box"},
		{"add", "box-front-sphere"},
		{"remove", "box-front-sphere"},
		{"add", "box-front-capsule"},
		{"remove", "box-front-capsule"},
	}

	for {
		for _, step := range sequence {
			select {
			case <-w.worldStateStore.streamCtx.Done():
				return
			default:
			}

			switch step.action {
			case "add":
				w.addDynamicBox(step.name)
			case "remove":
				w.removeDynamicBox(step.name)
			}

			select {
			case <-w.worldStateStore.streamCtx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
}

func (w *MovingGeosWorld) addDynamicBox(name string) {
	var xOffset float64

	switch name {
	case "box-front-box":
		xOffset = -5000 // In front of the main box
	case "box-front-sphere":
		xOffset = 0 // In front of the sphere
	case "box-front-capsule":
		xOffset = 5000 // In front of the capsule
	}

	uuid := name + "-" + time.Now().Format("20060102150405")
	transform := &commonpb.Transform{
		ReferenceFrame: "dynamic-box",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose: &commonpb.Pose{
				X: xOffset, Y: -2000, Z: 0, Theta: 0, OX: 0, OY: 0, OZ: 1,
			},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{
					DimsMm: &commonpb.Vector3{
						X: 500,
						Y: 500,
						Z: 500,
					},
				},
			},
		},
		Uuid:     []byte(uuid),
		Metadata: dynamicBoxMetadata,
	}

	w.worldStateStore.mu.Lock()
	w.worldStateStore.transforms[uuid] = transform
	w.worldStateStore.mu.Unlock()

	w.worldStateStore.emitTransformChange(transform, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED, nil)
}

func (w *MovingGeosWorld) removeDynamicBox(name string) {
	w.worldStateStore.mu.Lock()

	var uuidToRemove string
	for uuid := range w.worldStateStore.transforms {
		if strings.HasPrefix(uuid, name) {
			uuidToRemove = uuid
			break
		}
	}

	if uuidToRemove == "" {
		w.worldStateStore.mu.Unlock()
		return
	}

	transform := w.worldStateStore.transforms[uuidToRemove]
	delete(w.worldStateStore.transforms, uuidToRemove)
	w.worldStateStore.mu.Unlock()

	change := worldstatestore.TransformChange{
		ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
		Transform: &commonpb.Transform{
			Uuid: transform.Uuid,
		},
	}

	select {
	case w.worldStateStore.changeChan <- change:
	case <-w.worldStateStore.streamCtx.Done():
	default:
		// Channel is full, skip this update
	}
}

func (w *MovingGeosWorld) updateTransforms() {
	elapsed := time.Since(w.worldStateStore.startTime)

	w.updateBoxTransform(elapsed)
	w.updateSphereTransform(elapsed)
	w.updateCapsuleTransform(elapsed)
}
