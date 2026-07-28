package fake

import (
	"fmt"
	"math"
	"time"

	commonpb "go.viam.com/api/common/v1"
)

const (
	lidarObstacleCount = 6
	lidarRadiusMM      = 1000.0
	lidarObstacleZMM   = 200.0
)

// LidarWorld simulates a lidar detecting obstacles in the surrounding area: it reports a ring of
// upright obstacles around the robot and slowly rotates them each scan to mimic a moving field of
// returns. No real lidar is involved — the returns are synthesized.
type LidarWorld struct {
	worldStateStore *WorldStateStore
}

func lidarObstacleName(i int) string {
	return fmt.Sprintf("lidar-return-%d", i)
}

// obstaclePose places obstacle i evenly around a ring, rotated by an angular offset that advances with
// elapsed time.
func obstaclePose(i int, elapsedSeconds float64) *commonpb.Pose {
	angle := (2*math.Pi/float64(lidarObstacleCount))*float64(i) + elapsedSeconds*0.2
	return poseAt(lidarRadiusMM*math.Cos(angle), lidarRadiusMM*math.Sin(angle), lidarObstacleZMM)
}

// StartWorld seeds the initial ring of returns and starts the scan loop.
func (w *LidarWorld) StartWorld() {
	f := w.worldStateStore

	f.mu.Lock()
	for i := 0; i < lidarObstacleCount; i++ {
		name := lidarObstacleName(i)
		f.transforms[name] = f.newObstacle(name, obstaclePose(i, 0), capsuleGeometry(75, 400), colorMetadata(200, 200, 200, 0.8))
	}
	f.mu.Unlock()

	f.activeBackgroundWorkers.Add(1)
	go func() {
		defer f.activeBackgroundWorkers.Done()
		f.runCaptureLoop(w.onScan)
	}()
}

// onScan advances the ring one step per scan, emitting each return's new pose as an UPDATED change.
func (w *LidarWorld) onScan(elapsed time.Duration) {
	f := w.worldStateStore
	for i := 0; i < lidarObstacleCount; i++ {
		name := lidarObstacleName(i)
		pose := obstaclePose(i, elapsed.Seconds())

		f.mu.Lock()
		tf, ok := f.transforms[name]
		if ok {
			tf.PoseInObserverFrame.Pose.X = pose.X
			tf.PoseInObserverFrame.Pose.Y = pose.Y
		}
		f.mu.Unlock()
		if !ok {
			continue
		}

		f.emitTransformUpdate(&commonpb.Transform{
			Uuid:                []byte(name),
			PoseInObserverFrame: &commonpb.PoseInFrame{Pose: pose},
		}, []string{"poseInObserverFrame.pose.x", "poseInObserverFrame.pose.y"})
	}
}
