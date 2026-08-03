package fake

import (
	"math"
	"time"

	"github.com/golang/geo/r3"
	"github.com/viam-labs/motion-tools/draw"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/pointcloud"
)

const (
	lidarScanUUID = "lidar-scan"
	lidarRadiusMM = 500.0 // ring distance; within the demo arm's reach so it actually collides
	lidarSurfaces = 6     // number of obstacle surfaces the beam hits around the ring
	// lidarBearingOffset rotates the whole ring so a gap falls on the demo arm's home crossing (its upper
	// arm reaches out along +x), leaving the arm clear of every surface at startup.
	lidarBearingOffset = math.Pi / 6
	lidarArcHalf       = 0.20  // half the angular width of each surface, radians
	lidarArcSteps      = 9     // points sampled across each surface arc
	lidarZBaseMM       = 20.0  // lowest scanned height
	lidarZSpanMM       = 350.0 // vertical extent of each surface
	lidarZSteps        = 14    // points sampled up each surface
	lidarJitterMM      = 12.0  // range noise amplitude, simulating lidar measurement tolerance
	lidarJitterHz      = 1.6   // how fast the range noise jostles
)

// LidarWorld simulates a fixed 2D-ish lidar at the origin scanning stationary obstacles arranged in a
// ring around the robot. Each scan is a single point cloud of the surface points the beam hits, with a
// little per-scan range jitter for realism. No real lidar is involved; the returns are synthesized.
type LidarWorld struct {
	worldStateStore *WorldStateStore
}

// lidarRingCloud builds the points a lidar would report at time t: for each stationary surface around the
// ring, a vertical patch of hit points, each nudged in range by a small time-varying jitter. Points are
// in the world-centered frame; the surfaces do not move, only the range noise changes.
func lidarRingCloud(elapsedSeconds float64) pointcloud.PointCloud {
	cloud := pointcloud.NewBasicPointCloud(lidarSurfaces * lidarArcSteps * lidarZSteps)
	idx := 0
	for s := range lidarSurfaces {
		bearing := lidarBearingOffset + 2*math.Pi*float64(s)/float64(lidarSurfaces)
		for ia := range lidarArcSteps {
			angle := bearing - lidarArcHalf + 2*lidarArcHalf*float64(ia)/float64(lidarArcSteps-1)
			// per-point range jitter: a smooth jostle keyed to the point index so points do not move in lockstep
			radius := lidarRadiusMM + lidarJitterMM*math.Sin(elapsedSeconds*lidarJitterHz+float64(idx)*0.7)
			for iz := range lidarZSteps {
				z := lidarZBaseMM + lidarZSpanMM*float64(iz)/float64(lidarZSteps-1)
				// Set only errors on an invalid point; all of these are valid.
				_ = cloud.Set(r3.Vector{X: radius * math.Cos(angle), Y: radius * math.Sin(angle), Z: z}, pointcloud.NewValueData(100))
				idx++
			}
		}
	}
	return cloud
}

func (w *LidarWorld) scanTransform(elapsedSeconds float64) *commonpb.Transform {
	f := w.worldStateStore
	return f.newPointCloudObstacle(lidarScanUUID, poseAt(0, 0, 0), lidarRingCloud(elapsedSeconds), draw.ColorFromRGB(0, 0, 255))
}

// StartWorld emits the first scan and starts the scan loop.
func (w *LidarWorld) StartWorld() {
	f := w.worldStateStore

	tf := w.scanTransform(0)
	if tf == nil {
		return
	}
	f.mu.Lock()
	f.transforms[lidarScanUUID] = tf
	f.mu.Unlock()

	f.activeBackgroundWorkers.Go(func() {
		f.runCaptureLoop(w.onScan)
	})
}

// onScan re-scans each tick: a fresh cloud with new range jitter, re-emitted as the full transform (the
// visualizer treats UPDATED as a full-state upsert, and the point data itself changed).
func (w *LidarWorld) onScan(elapsed time.Duration) {
	f := w.worldStateStore
	tf := w.scanTransform(elapsed.Seconds())
	if tf == nil {
		return
	}
	f.mu.Lock()
	f.transforms[lidarScanUUID] = tf
	f.mu.Unlock()
	f.emitTransformUpdate(tf, []string{"physicalObject"})
}
