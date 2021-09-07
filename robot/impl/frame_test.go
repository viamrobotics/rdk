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
	test.That(t, fs.GetFrame("pieceArm").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{500, 0, 300})
	test.That(t, fs.GetFrame("pieceArm_offset"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("pieceArm_offset").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{500, 500, 1000})
	test.That(t, fs.GetFrame("pieceGripper"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("pieceGripper").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{0, 0, 200})
	test.That(t, fs.GetFrame("pieceGripper_offset"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("pieceGripper_offset").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{0, 0, 0})
	test.That(t, fs.GetFrame("compass2"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("compass2").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{0, 0, 0})
	test.That(t, fs.GetFrame("compass2_offset"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("compass2_offset").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{0, 0, 0})
	test.That(t, fs.GetFrame("cameraOver"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("cameraOver").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{0, 0, 0})
	test.That(t, fs.GetFrame("cameraOver_offset"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("cameraOver_offset").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{2000, 500, 1300})
	test.That(t, fs.GetFrame("lidar1"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("lidar1").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{50, 0, 0})
	test.That(t, fs.GetFrame("lidar1_offset"), test.ShouldNotBeNil)
	test.That(t, fs.GetFrame("lidar1_offset").Transform(emptyIn).Point(), test.ShouldResemble, r3.Vector{0, 0, 200})
	test.That(t, fs.GetFrame("compass1"), test.ShouldBeNil) // compass1 is not registered

	// There is a point at (1500, 500, 1300) in the world frame. See if it transforms correctly in each frame.
	worldPt := r3.Vector{1500, 500, 1300}
	armPt := r3.Vector{0, 0, 500}
	transformPoint, err := fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("pieceArm"))
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, armPt.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, armPt.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, armPt.Z)

	sensorPt := r3.Vector{0, 0, 500}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("compass2"))
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, sensorPt.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, sensorPt.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, sensorPt.Z)

	gripperPt := r3.Vector{0, 0, 300}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("pieceGripper"))
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, gripperPt.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, gripperPt.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, gripperPt.Z)

	cameraPt := r3.Vector{500, 0, 0}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("cameraOver"))
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, cameraPt.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, cameraPt.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, cameraPt.Z)

	lidarPt := r3.Vector{450, 0, -200}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, fs.World(), fs.GetFrame("lidar1"))
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, lidarPt.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, lidarPt.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, lidarPt.Z)

	// go from camera point to gripper point
	transformPoint, err = fs.TransformPoint(blankPos, cameraPt, fs.GetFrame("cameraOver"), fs.GetFrame("pieceGripper"))
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, gripperPt.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, gripperPt.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, gripperPt.Z)

}
