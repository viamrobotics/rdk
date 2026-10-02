package fake

import (
	"math"
	"time"

	"github.com/viam-labs/motion-tools/draw"
	commonpb "go.viam.com/api/common/v1"
)

// blockSizeMM is the side length of each detected block. Sized so an arm reaching toward a block visibly
// collides with it, making the store-driven obstacle avoidance obvious in the visualizer.
const blockSizeMM = 150

// DepthCameraWorld simulates a depth camera detecting three colored blocks on a table and reporting them
// into the store, refreshed each capture cycle. No real camera or vision service is involved.
type DepthCameraWorld struct {
	worldStateStore *WorldStateStore
}

type detectedBlock struct {
	name                string
	baseX, baseY, baseZ float64
	color               draw.Color
}

// blocks are three colored boxes side-by-side in front of the camera, resting on the floor (center at
// half the block height) with a gap between them.
func (w *DepthCameraWorld) blocks() []detectedBlock {
	return []detectedBlock{
		{"red-block", -250, 400, blockSizeMM / 2, draw.ColorFromRGB(255, 0, 0)},
		{"green-block", 0, 400, blockSizeMM / 2, draw.ColorFromRGB(0, 255, 0)},
		{"blue-block", 250, 400, blockSizeMM / 2, draw.ColorFromRGB(0, 0, 255)},
	}
}

// StartWorld seeds the blocks and starts the capture loop.
func (w *DepthCameraWorld) StartWorld() {
	f := w.worldStateStore

	f.mu.Lock()
	for _, b := range w.blocks() {
		geom, err := boxGeometry(b.name, blockSizeMM, blockSizeMM, blockSizeMM)
		if err != nil {
			f.logger.Errorf("world state store: %v", err)
			continue
		}
		if tf := f.newObstacle(b.name, poseAt(b.baseX, b.baseY, b.baseZ), geom, b.color); tf != nil {
			f.transforms[b.name] = tf
		}
	}
	f.mu.Unlock()

	f.activeBackgroundWorkers.Go(func() {
		f.runCaptureLoop(w.onCapture)
	})
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
