package fake

import (
	"time"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func identityPose() *commonpb.Pose {
	return &commonpb.Pose{OZ: 1}
}

func poseAt(x, y, z float64) *commonpb.Pose {
	return &commonpb.Pose{X: x, Y: y, Z: z, OZ: 1}
}

func boxGeometry(x, y, z float64) *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: identityPose(),
		GeometryType: &commonpb.Geometry_Box{
			Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{X: x, Y: y, Z: z}},
		},
	}
}

func capsuleGeometry(radiusMM, lengthMM float64) *commonpb.Geometry {
	return &commonpb.Geometry{
		Center: identityPose(),
		GeometryType: &commonpb.Geometry_Capsule{
			Capsule: &commonpb.Capsule{RadiusMm: radiusMM, LengthMm: lengthMM},
		},
	}
}

// colorMetadata builds the color+opacity rendering hints the client visualizer reads.
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

// newObstacle builds a world state store transform for a detected obstacle. The geometry gets an
// explicit identity center so consumers like the motion service can use it directly.
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

// runCaptureLoop calls onTick at the store's fps, re-reading fps between ticks so a DoCommand fps change
// takes effect, until the stream context is canceled.
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
