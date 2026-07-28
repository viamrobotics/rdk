package fake

import (
	"time"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// identityPose returns a zero-translation, identity-orientation pose.
func identityPose() *commonpb.Pose {
	return &commonpb.Pose{OZ: 1}
}

// poseAt returns a translation-only pose (identity orientation) at the given millimeter coordinates.
func poseAt(x, y, z float64) *commonpb.Pose {
	return &commonpb.Pose{X: x, Y: y, Z: z, OZ: 1}
}

// boxGeometry returns a box geometry of the given millimeter dimensions centered at its frame origin.
func boxGeometry(x, y, z float64) *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: identityPose(),
		GeometryType: &commonpb.Geometry_Box{
			Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{X: x, Y: y, Z: z}},
		},
	}
}

// capsuleGeometry returns a capsule geometry centered at its frame origin.
func capsuleGeometry(radiusMM, lengthMM float64) *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: identityPose(),
		GeometryType: &commonpb.Geometry_Capsule{
			Capsule: &commonpb.Capsule{RadiusMm: radiusMM, LengthMm: lengthMM},
		},
	}
}

// colorMetadata builds the rendering-hint metadata (color + opacity) used by the client visualizer.
func colorMetadata(r, g, b, opacity float64) *structpb.Struct {
	s, err := structpb.NewStruct(map[string]any{
		"color":   map[string]any{"r": r, "g": g, "b": b},
		"opacity": opacity,
	})
	if err != nil {
		return nil
	}
	return s
}

// newObstacle builds a fully-formed world state store transform for a detected obstacle: a geometry at
// `pose` within the store's configured parent frame, keyed and named by `name`. The geometry carries an
// explicit identity center so downstream consumers (e.g. the motion service) can use it directly.
func (f *WorldStateStore) newObstacle(
	name string,
	pose *commonpb.Pose,
	geometry *commonpb.Geometry,
	metadata *structpb.Struct,
) *commonpb.Transform {
	geometry.Label = name
	return &commonpb.Transform{
		ReferenceFrame: name,
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: f.parentFrame,
			Pose:           pose,
		},
		PhysicalObject: geometry,
		Uuid:           []byte(name),
		Metadata:       metadata,
	}
}

// runCaptureLoop invokes onTick at the store's capture rate (fps), re-reading fps between ticks so a
// DoCommand fps change is honored, until the store's stream context is canceled. onTick receives the
// elapsed time since the store started.
func (f *WorldStateStore) runCaptureLoop(onTick func(elapsed time.Duration)) {
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
			onTick(time.Since(f.startTime))

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
