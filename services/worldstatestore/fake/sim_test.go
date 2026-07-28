package fake

import (
	"context"
	"errors"
	"io"
	"sort"
	"testing"
	"time"

	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

func newSim(t *testing.T, sensorType string) *WorldStateStore {
	t.Helper()
	name := resource.NewName(worldstatestore.API, "sim")
	svc := newFakeWorldStateStore(name, &Config{InputSensorType: sensorType}, logging.NewTestLogger(t))
	return svc.(*WorldStateStore)
}

func uuidStrings(t *testing.T, svc worldstatestore.Service) []string {
	t.Helper()
	uuids, err := svc.ListUUIDs(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	out := make([]string, 0, len(uuids))
	for _, u := range uuids {
		out = append(out, string(u))
	}
	sort.Strings(out)
	return out
}

func TestDepthCameraWorld(t *testing.T) {
	sim := newSim(t, "depth_camera")
	defer sim.Close(context.Background())

	test.That(t, uuidStrings(t, sim), test.ShouldResemble, []string{"blue-block", "green-block", "red-block"})

	for _, name := range []string{"red-block", "green-block", "blue-block"} {
		tf, err := sim.GetTransform(context.Background(), []byte(name), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, tf.GetPoseInObserverFrame().GetReferenceFrame(), test.ShouldEqual, "world")
		test.That(t, tf.GetPhysicalObject().GetBox(), test.ShouldNotBeNil)
		// A fully-formed geometry (non-nil center) so consumers like the motion service can use it directly.
		test.That(t, tf.GetPhysicalObject().GetCenter(), test.ShouldNotBeNil)
		test.That(t, tf.GetMetadata(), test.ShouldNotBeNil)

		// The transform round-trips into a reference-frame link (the motion consume path).
		link, err := referenceframe.LinkInFrameFromTransformProtobuf(tf)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, link.Geometry(), test.ShouldNotBeNil)
	}
}

func TestDepthCameraWorldParentFrameOverride(t *testing.T) {
	name := resource.NewName(worldstatestore.API, "sim")
	svc := newFakeWorldStateStore(name, &Config{InputSensorType: "depth_camera", ParentFrame: "base"}, logging.NewTestLogger(t))
	defer svc.Close(context.Background())

	tf, err := svc.GetTransform(context.Background(), []byte("red-block"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, tf.GetPoseInObserverFrame().GetReferenceFrame(), test.ShouldEqual, "base")
}

func TestLidarWorld(t *testing.T) {
	sim := newSim(t, "lidar")
	defer sim.Close(context.Background())

	uuids := uuidStrings(t, sim)
	test.That(t, len(uuids), test.ShouldEqual, lidarObstacleCount)

	tf, err := sim.GetTransform(context.Background(), []byte(lidarObstacleName(0)), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, tf.GetPhysicalObject().GetCapsule(), test.ShouldNotBeNil)
	test.That(t, tf.GetPhysicalObject().GetCenter(), test.ShouldNotBeNil)
}

// TestStreamSnapshotAsAdded verifies a late subscriber receives the full current world as ADDED changes.
func TestStreamSnapshotAsAdded(t *testing.T) {
	sim := newSim(t, "depth_camera")
	defer sim.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := sim.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	seen := make(map[string]bool)
	for len(seen) < 3 {
		change, err := stream.Next()
		test.That(t, err, test.ShouldBeNil)
		if change.ChangeType == pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED {
			seen[string(change.Transform.GetUuid())] = true
		}
	}
	test.That(t, seen, test.ShouldContainKey, "red-block")
	test.That(t, seen, test.ShouldContainKey, "green-block")
	test.That(t, seen, test.ShouldContainKey, "blue-block")
}

// TestMultipleSubscribersEachGetSnapshot verifies the broadcaster-backed stream serves concurrent
// subscribers independently (the old single-channel implementation could not).
func TestMultipleSubscribersEachGetSnapshot(t *testing.T) {
	sim := newSim(t, "lidar")
	defer sim.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	countSnapshot := func() int {
		stream, err := sim.StreamTransformChanges(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		added := 0
		for added < lidarObstacleCount {
			change, err := stream.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			test.That(t, err, test.ShouldBeNil)
			if change.ChangeType == pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED {
				added++
			}
		}
		return added
	}

	test.That(t, countSnapshot(), test.ShouldEqual, lidarObstacleCount)
	test.That(t, countSnapshot(), test.ShouldEqual, lidarObstacleCount)
}
