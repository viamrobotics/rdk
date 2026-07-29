package fake

import (
	"math"
	"time"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// DepthCameraWorld simulates a depth camera detecting three colored blocks on a table and reporting them
// into the store, refreshed each capture cycle. No real camera or vision service is involved.
type DepthCameraWorld struct {
	worldStateStore *WorldStateStore
}

type detectedBlock struct {
	name              string
	baseX, baseY, ldZ float64
	metadata          *structpb.Struct
}

// blocks are three colored boxes side-by-side on a table in front of the camera.
func (w *DepthCameraWorld) blocks() []detectedBlock {
	return []detectedBlock{
		{"red-block", -150, 400, 25, colorMetadata(255, 0, 0, 1)},
		{"green-block", 0, 400, 25, colorMetadata(0, 255, 0, 1)},
		{"blue-block", 150, 400, 25, colorMetadata(0, 0, 255, 1)},
	}
}

// StartWorld starts the depth camera simulation.
func (w *DepthCameraWorld) StartWorld() {
	f := w.worldStateStore

	f.mu.Lock()
	for _, b := range w.blocks() {
		f.transforms[b.name] = f.newObstacle(b.name, poseAt(b.baseX, b.baseY, b.ldZ), boxGeometry(50, 50, 50), b.metadata)
	}
	f.mu.Unlock()

	f.activeBackgroundWorkers.Add(1)
	go func() {
		defer f.activeBackgroundWorkers.Done()
		f.runCaptureLoop(w.onCapture)
	}()
}

// onCapture re-detects the same blocks each cycle with a little positional noise (UPDATED changes).
func (w *DepthCameraWorld) onCapture(elapsed time.Duration) {
	f := w.worldStateStore
	for i, b := range w.blocks() {
		phase := elapsed.Seconds() + float64(i)
		x := b.baseX + math.Sin(phase)*5
		y := b.baseY + math.Cos(phase)*5

		f.mu.Lock()
		tf, ok := f.transforms[b.name]
		if ok {
			tf.PoseInObserverFrame.Pose.X = x
			tf.PoseInObserverFrame.Pose.Y = y
		}
		f.mu.Unlock()
		if !ok {
			continue
		}

		f.emitTransformUpdate(&commonpb.Transform{
			Uuid: []byte(b.name),
			PoseInObserverFrame: &commonpb.PoseInFrame{
				Pose: &commonpb.Pose{X: x, Y: y},
			},
		}, []string{"poseInObserverFrame.pose.x", "poseInObserverFrame.pose.y"})
	}
}
