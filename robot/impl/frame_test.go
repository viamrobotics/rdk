package robotimpl_test

import (
	"context"
	"testing"

	"go.viam.com/core/config"
	"go.viam.com/core/referenceframe"
	robotimpl "go.viam.com/core/robot/impl"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"github.com/golang/geo/r3"
)

var blankPos map[string][]referenceframe.Input

func TestFrameSystemFromConfig(t *testing.T) {
	// use impl/data/fake.json as config input
	emptyIn := []referenceframe.Input{}
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read("data/fake.json")
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// use fake registrations to have a FrameSystem return
	fs, err := r.FrameSystem(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(fs.Frames()), test.ShouldEqual, 10) // 5 frames defined, 10 frames when including the offset

	// see if all frames are present and if their frames are correct
	test.That(t, fs.GetFrame("world"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("pieceArm"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("pieceArm").Transform(emptyIn).Point(), r3.Vector{500, 0, 300})
	test.That(t, fs.GetFrame("pieceArm_offset"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("pieceArm_offset").Transform(emptyIn).Point(), r3.Vector{500, 500, 1000})
	test.That(t, fs.GetFrame("pieceGripper"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("pieceGripper").Transform(emptyIn).Point(), r3.Vector{0, 0, 200})
	test.That(t, fs.GetFrame("pieceGripper_offset"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("pieceGripper_offset").Transform(emptyIn).Point(), r3.Vector{0, 0, 0})
	test.That(t, fs.GetFrame("compass2"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("compass2").Transform(emptyIn).Point(), r3.Vector{0, 0, 0})
	test.That(t, fs.GetFrame("compass2_offset"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("compass2_offset").Transform(emptyIn).Point(), r3.Vector{0, 0, 0})
	test.That(t, fs.GetFrame("cameraOver"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("cameraOver").Transform(emptyIn).Point(), r3.Vector{0, 0, 0})
	test.That(t, fs.GetFrame("cameraOver_offset"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("cameraOver_offset").Transform(emptyIn).Point(), r3.Vector{2000, 500, 1300})
	test.That(t, fs.GetFrame("lidar1"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("lidar1").Transform(emptyIn).Point(), r3.Vector{50, 0, 0})
	test.That(t, fs.GetFrame("lidar1_offset"), test.ShouldNotBeNil)
	pointAlmostEqual(t, fs.GetFrame("lidar1_offset").Transform(emptyIn).Point(), r3.Vector{0, 0, 200})
	test.That(t, fs.GetFrame("compass1"), test.ShouldBeNil) // compass1 is not registered

	// There is a point at (1500, 500, 1300) in the world frame. See if it transforms correctly in each frame.
	worldPt := r3.Vector{1500, 500, 1300}
	armPt := r3.Vector{0, 0, 500}
	transformPoint, err := fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("pieceArm"))
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, armPt)

	sensorPt := r3.Vector{0, 0, 500}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("compass2"))
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, sensorPt)

	gripperPt := r3.Vector{0, 0, 300}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("pieceGripper"))
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, gripperPt)

	cameraPt := r3.Vector{500, 0, 0}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("cameraOver"))
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, cameraPt)

	lidarPt := r3.Vector{450, 0, -200}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("lidar1"))
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, lidarPt)

	// go from camera point to gripper point
	transformPoint, err = fs.TransformPoint(blankPos, cameraPt, fs.GetFrame("cameraOver"), fs.GetFrame("pieceGripper"))
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, gripperPt)

}

// All of these config files should fail
func TestWrongFrameSystems(t *testing.T) {
	// use impl/data/fake_wrongconfig*.json as config input
	logger := golog.NewTestLogger(t)

	cfg, err := config.Read("data/fake_wrongconfig1.json") // has disconnected components (misspelled parent)
	test.That(t, err, test.ShouldBeNil)
	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, err = r.FrameSystem(context.Background())
	test.That(t, err, test.ShouldNotBeNil)

	cfg, err = config.Read("data/fake_wrongconfig2.json") // no world node
	test.That(t, err, test.ShouldBeNil)
	r, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, err = r.FrameSystem(context.Background())
	test.That(t, err, test.ShouldNotBeNil)

	cfg, err = config.Read("data/fake_wrongconfig3.json") // there is a cycle in the graph
	test.That(t, err, test.ShouldBeNil)
	r, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, err = r.FrameSystem(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
}

func pointAlmostEqual(t *testing.T, from, to r3.Vector) {
	test.That(t, from.X, test.ShouldAlmostEqual, to.X)
	test.That(t, from.Y, test.ShouldAlmostEqual, to.Y)
	test.That(t, from.Z, test.ShouldAlmostEqual, to.Z)
}
