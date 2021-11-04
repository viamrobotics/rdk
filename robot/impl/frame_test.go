package robotimpl_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/config"
	"go.viam.com/core/referenceframe"
	robotimpl "go.viam.com/core/robot/impl"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
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
	defer r.Close()

	// use fake registrations to have a FrameSystem return
	fs, err := r.FrameSystem(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(fs.FrameNames()), test.ShouldEqual, 10) // 5 frames defined, 10 frames when including the offset

	// see if all frames are present and if their frames are correct
	test.That(t, fs.GetFrame("world"), test.ShouldNotBeNil)

	test.That(t, fs.GetFrame("pieceArm"), test.ShouldNotBeNil)
	pose, err := fs.GetFrame("pieceArm").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{500, 0, 300})

	test.That(t, fs.GetFrame("pieceArm_offset"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("pieceArm_offset").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{500, 500, 1000})

	test.That(t, fs.GetFrame("pieceGripper"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("pieceGripper").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 200})

	test.That(t, fs.GetFrame("pieceGripper_offset"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("pieceGripper_offset").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	test.That(t, fs.GetFrame("compass2"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("compass2").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	test.That(t, fs.GetFrame("compass2_offset"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("compass2_offset").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	test.That(t, fs.GetFrame("cameraOver"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("cameraOver").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	test.That(t, fs.GetFrame("cameraOver_offset"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("cameraOver_offset").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{2000, 500, 1300})

	test.That(t, fs.GetFrame("lidar1"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("lidar1").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{50, 0, 0})

	test.That(t, fs.GetFrame("lidar1_offset"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("lidar1_offset").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 200})

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
	test.That(t, err, test.ShouldBeError, errors.New("the frame system is not fully connected, expected 11 frames but frame system has 9. Expected frames are: [cameraOver cameraOver_offset compass2 compass2_offset lidar1 lidar1_offset pieceArm pieceArm_offset pieceGripper pieceGripper_offset world]. Actual frames are: [cameraOver cameraOver_offset lidar1 lidar1_offset pieceArm pieceArm_offset pieceGripper pieceGripper_offset world]"))
	test.That(t, r.Close(), test.ShouldBeNil)

	cfg, err = config.Read("data/fake_wrongconfig2.json") // no world node
	test.That(t, err, test.ShouldBeNil)
	r, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, err = r.FrameSystem(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'"))
	test.That(t, r.Close(), test.ShouldBeNil)

	cfg, err = config.Read("data/fake_wrongconfig3.json") // one of the nodes was given the name world
	test.That(t, err, test.ShouldBeNil)
	r, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, err = r.FrameSystem(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("cannot have more than one frame with name world"))
	test.That(t, r.Close(), test.ShouldBeNil)

	cfg, err = config.Read("data/fake_wrongconfig4.json") // the parent field was left empty for a component
	test.That(t, err, test.ShouldBeNil)
	r, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, err = r.FrameSystem(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("parent field in component \"cameraOver\" is empty"))
	test.That(t, r.Close(), test.ShouldBeNil)
}

func pointAlmostEqual(t *testing.T, from, to r3.Vector) {
	test.That(t, from.X, test.ShouldAlmostEqual, to.X)
	test.That(t, from.Y, test.ShouldAlmostEqual, to.Y)
	test.That(t, from.Z, test.ShouldAlmostEqual, to.Z)
}
