package fake

import (
	"time"

	"github.com/golang/geo/r3"
	"github.com/viam-labs/motion-tools/draw"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

// poseAt returns a translation-only pose at the given millimeter coordinates.
func poseAt(x, y, z float64) spatialmath.Pose {
	return spatialmath.NewPoseFromPoint(r3.Vector{X: x, Y: y, Z: z})
}

func boxGeometry(name string, x, y, z float64) (spatialmath.Geometry, error) {
	return spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: x, Y: y, Z: z}, name)
}

// newObstacle builds a detection transform through the draw API so its geometry, pose, and color
// render correctly in the motion-tools visualizer. The uuid is the obstacle name for stable keying.
func (f *WorldStateStore) newObstacle(
	name string,
	pose spatialmath.Pose,
	geometry spatialmath.Geometry,
	color draw.Color,
) *commonpb.Transform {
	drawn, err := draw.NewDrawnGeometry(geometry, draw.WithGeometryColor(color))
	if err != nil {
		f.logger.Errorf("world state store: failed to build geometry %q: %v", name, err)
		return nil
	}
	transform, err := drawn.Draw(name, draw.WithParent(f.parentFrame), draw.WithPose(pose), draw.WithUUID([]byte(name)))
	if err != nil {
		f.logger.Errorf("world state store: failed to draw obstacle %q: %v", name, err)
		return nil
	}
	return transform
}

// newPointCloudObstacle builds a detection transform from a point cloud through the draw API. Draw
// serializes the cloud as an octree, which participates in collision detection like any other geometry.
func (f *WorldStateStore) newPointCloudObstacle(
	name string,
	pose spatialmath.Pose,
	cloud pointcloud.PointCloud,
	color draw.Color,
) *commonpb.Transform {
	drawn, err := draw.NewDrawnPointCloud(cloud, draw.WithSinglePointCloudColor(color))
	if err != nil {
		f.logger.Errorf("world state store: failed to build point cloud %q: %v", name, err)
		return nil
	}
	transform, err := drawn.Draw(name, draw.WithParent(f.parentFrame), draw.WithPose(pose), draw.WithUUID([]byte(name)))
	if err != nil {
		f.logger.Errorf("world state store: failed to draw point cloud obstacle %q: %v", name, err)
		return nil
	}
	return transform
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
