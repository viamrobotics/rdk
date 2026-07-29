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

// LidarWorld simulates a lidar reporting a ring of obstacles around the robot, slowly rotated each scan.
// No real lidar is involved — the returns are synthesized.
type LidarWorld struct {
	worldStateStore *WorldStateStore
}

func lidarObstacleName(i int) string {
	return fmt.Sprintf("lidar-return-%d", i)
}

// obstaclePose places obstacle i around a ring, rotated by an offset that advances with time.
func obstaclePose(i int, elapsedSeconds float64) *commonpb.Pose {
	angle := (2*math.Pi/float64(lidarObstacleCount))*float64(i) + elapsedSeconds*0.2
	return poseAt(lidarRadiusMM*math.Cos(angle), lidarRadiusMM*math.Sin(angle), lidarObstacleZMM)
}

// StartWorld starts the lidar simulation.
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

// onScan advances the ring each scan, emitting each return's new pose (UPDATED changes).
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
