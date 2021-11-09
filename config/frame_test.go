package config

import (
	"encoding/json"
	"io/ioutil"
	"math"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"github.com/go-errors/errors"
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/num/quat"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	coreutils "go.viam.com/core/utils"
)

func TestOrientation(t *testing.T) {
	file, err := os.Open("data/frames.json")
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(file.Close)

	data, err := ioutil.ReadAll(file)
	test.That(t, err, test.ShouldBeNil)
	// Parse into map of tests
	var testMap map[string]json.RawMessage
	err = json.Unmarshal(data, &testMap)
	test.That(t, err, test.ShouldBeNil)
	// go through each test case

	// Config with unknown orientation
	frame := Frame{}
	err = json.Unmarshal(testMap["wrong"], &frame)
	test.That(t, err, test.ShouldBeError, errors.New("orientation type oiler_angles not recognized"))

	// Empty Config
	frame = Frame{}
	err = json.Unmarshal(testMap["empty"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{0, 0, 0})
	test.That(t, frame.Orientation.Quaternion(), test.ShouldResemble, quat.Number{1, 0, 0, 0})

	pose := frame.Pose()
	test.That(t, pose, test.ShouldResemble, spatial.NewZeroPose())
	staticFrame, err := frame.StaticFrame("test")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, staticFrame, test.ShouldResemble, referenceframe.NewZeroStaticFrame("test"))

	// Mostly Empty Config
	frame = Frame{}
	err = json.Unmarshal(testMap["mostlyempty"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "a")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{0, 0, 0})
	test.That(t, frame.Orientation.Quaternion(), test.ShouldResemble, quat.Number{1, 0, 0, 0})

	// OrientationVectorDegrees Config
	frame = Frame{}
	err = json.Unmarshal(testMap["ovdegrees"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "a")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{1, 2, 3})
	test.That(t, frame.Orientation.OrientationVectorDegrees(), test.ShouldResemble, &spatial.OrientationVectorDegrees{45, 0, 0, 1})

	// OrientationVector Radians Config
	frame = Frame{}
	err = json.Unmarshal(testMap["ovradians"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "b")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{4, 5, 6})
	test.That(t, frame.Orientation.OrientationVectorRadians(), test.ShouldResemble, &spatial.OrientationVector{0.78539816, 0, 1, 0})

	// Euler Angles
	frame = Frame{}
	err = json.Unmarshal(testMap["euler"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "c")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{7, 8, 9})
	test.That(t, frame.Orientation.EulerAngles(), test.ShouldResemble, &spatial.EulerAngles{Roll: 0, Pitch: 0, Yaw: 45})

	// Axis angles Config
	frame = Frame{}
	err = json.Unmarshal(testMap["axisangle"], &frame)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frame.Parent, test.ShouldEqual, "d")
	test.That(t, frame.Translation, test.ShouldResemble, Translation{0, 0, 0})
	test.That(t, frame.Orientation.AxisAngles(), test.ShouldResemble, &spatial.R4AA{0.78539816, 1, 0, 0})
}

func TestFrameModelPart(t *testing.T) {
	jsonData, err := ioutil.ReadFile(coreutils.ResolveFile("config/data/model_frame.json"))
	test.That(t, err, test.ShouldBeNil)

	// minimally specified part
	part := &FrameSystemPart{
		Name:             "test",
		FrameConfig:      nil,
		ModelFrameConfig: nil,
	}
	result := part.ToProtobuf()
	test.That(t, result, test.ShouldBeNil)

	// slightly specified part
	part = &FrameSystemPart{
		Name:             "test",
		FrameConfig:      &Frame{Parent: "world"},
		ModelFrameConfig: nil,
	}
	result = part.ToProtobuf()
	pose := &pb.Pose{OZ: 1, Theta: 0} // zero pose
	exp := &pb.FrameSystemConfig{Name: "test", FrameConfig: &pb.FrameConfig{Parent: "world", Pose: pose}}
	test.That(t, result.String(), test.ShouldResemble, exp.String())

	// fully specified part
	part = &FrameSystemPart{
		Name:             "test",
		FrameConfig:      &Frame{Parent: "world", Translation: Translation{1, 2, 3}, Orientation: spatial.NewZeroOrientation()},
		ModelFrameConfig: jsonData,
	}
	result = part.ToProtobuf()
	pose = &pb.Pose{X: 1, Y: 2, Z: 3, OZ: 1, Theta: 0}
	exp = &pb.FrameSystemConfig{Name: "test", FrameConfig: &pb.FrameConfig{Parent: "world", Pose: pose}, ModelJson: jsonData}
	test.That(t, result.String(), test.ShouldResemble, exp.String())
}

func TestMergeFrameSystems(t *testing.T) {
	blankPos := map[string][]referenceframe.Input{}
	// build 2 frame systems
	fs1 := referenceframe.NewEmptySimpleFrameSystem("test1")
	fs2 := referenceframe.NewEmptySimpleFrameSystem("test2")

	frame1, err := referenceframe.NewStaticFrame("frame1", spatial.NewPoseFromPoint(r3.Vector{-5, 5, 0}))
	test.That(t, err, test.ShouldBeNil)
	err = fs1.AddFrame(frame1, fs1.World())
	test.That(t, err, test.ShouldBeNil)
	frame2, err := referenceframe.NewStaticFrame("frame2", spatial.NewPoseFromPoint(r3.Vector{0, 0, 10}))
	test.That(t, err, test.ShouldBeNil)
	err = fs1.AddFrame(frame2, fs1.GetFrame("frame1"))
	test.That(t, err, test.ShouldBeNil)

	// frame3 - pure translation
	frame3, err := referenceframe.NewStaticFrame("frame3", spatial.NewPoseFromPoint(r3.Vector{-2., 7., 1.}))
	test.That(t, err, test.ShouldBeNil)
	err = fs2.AddFrame(frame3, fs2.World())
	test.That(t, err, test.ShouldBeNil)
	// frame4 - pure rotiation around y 90 degrees
	frame4, err := referenceframe.NewStaticFrame("frame4", spatial.NewPoseFromAxisAngle(r3.Vector{}, r3.Vector{0., 1., 0.}, math.Pi/2))
	test.That(t, err, test.ShouldBeNil)
	err = fs2.AddFrame(frame4, fs2.GetFrame("frame3"))
	test.That(t, err, test.ShouldBeNil)

	// merge to fs1 with zero offset
	err = MergeFrameSystems(fs1, fs2, nil)
	test.That(t, err, test.ShouldBeNil)
	pointStart := r3.Vector{0, 0, 0}  // PoV from frame 2
	pointEnd := r3.Vector{-9, -2, -3} // PoV from frame 4
	transformPoint, err := fs1.TransformPoint(blankPos, pointStart, fs1.GetFrame("frame2"), fs1.GetFrame("frame4"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointEnd.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointEnd.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointEnd.Z)

	// reset fs1 framesystem to original
	fs1 = referenceframe.NewEmptySimpleFrameSystem("test1")
	err = fs1.AddFrame(frame1, fs1.World())
	test.That(t, err, test.ShouldBeNil)
	err = fs1.AddFrame(frame2, fs1.GetFrame("frame1"))
	test.That(t, err, test.ShouldBeNil)

	// merge to fs1 with an offset and rotation
	offsetConfig := &Frame{Parent: "frame1", Translation: Translation{1, 2, 3}, Orientation: &spatial.R4AA{Theta: math.Pi / 2, RZ: 1.}}
	err = MergeFrameSystems(fs1, fs2, offsetConfig)
	test.That(t, err, test.ShouldBeNil)
	// the frame of test2_world is rotated around z by 90 degrees, then displaced by (1,2,3) in the frame of frame1,
	// so the origin of frame1 from the perspective of test2_frame should be (-2, 1, -3)
	pointStart = r3.Vector{0, 0, 0} // PoV from frame 1
	pointEnd = r3.Vector{-2, 1, -3} // PoV from the world of test2
	transformPoint, err = fs1.TransformPoint(blankPos, pointStart, fs1.GetFrame("frame1"), fs1.GetFrame("test2_world"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointEnd.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointEnd.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointEnd.Z)
	// frame frame 2 to frame 4
	pointStart = r3.Vector{0, 0, 0} // PoV from frame 2
	pointEnd = r3.Vector{-6, -6, 0} // PoV from frame 4
	transformPoint, err = fs1.TransformPoint(blankPos, pointStart, fs1.GetFrame("frame2"), fs1.GetFrame("frame4"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformPoint.X, test.ShouldAlmostEqual, pointEnd.X)
	test.That(t, transformPoint.Y, test.ShouldAlmostEqual, pointEnd.Y)
	test.That(t, transformPoint.Z, test.ShouldAlmostEqual, pointEnd.Z)
}
