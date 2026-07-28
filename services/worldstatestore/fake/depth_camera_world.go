package fake

import (
	"math"
	"time"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// DepthCameraWorld simulates a depth camera running object detection (à la GetObjectPointClouds): it
// "detects" a red, a green, and a blue block sitting side-by-side on a table and reports them into the
// world state store, refreshing each block's pose on every capture cycle. No real camera or vision
// service is involved — the detections are synthesized.
type DepthCameraWorld struct {
	worldStateStore *WorldStateStore
}

type detectedBlock struct {
	name              string
	baseX, baseY, ldZ float64
	metadata          *structpb.Struct
}

// blocks returns the three colored blocks arranged side-by-side (X) a fixed distance in front (Y) of
// the camera, resting on a table surface (Z at half the block height).
func (w *DepthCameraWorld) blocks() []detectedBlock {
	return []detectedBlock{
		{"red-block", -150, 400, 25, colorMetadata(255, 0, 0, 1)},
		{"green-block", 0, 400, 25, colorMetadata(0, 255, 0, 1)},
		{"blue-block", 150, 400, 25, colorMetadata(0, 0, 255, 1)},
	}
}

// StartWorld seeds the initial detections and starts the capture loop.
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

// onCapture simulates a fresh detection each capture cycle: the same blocks are re-detected with a small
// amount of positional noise, emitted as UPDATED changes.
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
