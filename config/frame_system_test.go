package config

import (
	"math"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

func TestFrameModelPart(t *testing.T) {
	jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame.json"))
	test.That(t, err, test.ShouldBeNil)
	model, err := referenceframe.UnmarshalModelJSON(jsonData, "")
	test.That(t, err, test.ShouldBeNil)

	// minimally specified part
	part := &FrameSystemPart{
		Name:        "test",
		FrameConfig: nil,
		ModelFrame:  nil,
	}
	_, err = part.ToProtobuf()
	test.That(t, err, test.ShouldBeError, referenceframe.ErrNoModelInformation)

	// slightly specified part
	part = &FrameSystemPart{
		Name:        "test",
		FrameConfig: &Frame{Parent: "world"},
		ModelFrame:  nil,
	}
	result, err := part.ToProtobuf()
	test.That(t, err, test.ShouldBeNil)
	pose := &commonpb.Pose{OZ: 1, Theta: 0} // zero pose
	exp := &robotpb.FrameSystemConfig{
		Name: "test",
		PoseInParentFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose:           pose,
		},
	}
	test.That(t, result.Name, test.ShouldEqual, exp.Name)
	test.That(t, result.PoseInParentFrame, test.ShouldResemble, exp.PoseInParentFrame)
	test.That(t, result.ModelJson, test.ShouldResemble, exp.ModelJson)
	// return to FrameSystemPart
	partAgain, err := ProtobufToFrameSystemPart(result)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, partAgain.Name, test.ShouldEqual, part.Name)
	test.That(t, partAgain.FrameConfig.Parent, test.ShouldEqual, part.FrameConfig.Parent)
	test.That(t, partAgain.FrameConfig.Translation, test.ShouldResemble, part.FrameConfig.Translation)
	// nil orientations become specified as zero orientations
	test.That(t, partAgain.FrameConfig.Orientation, test.ShouldResemble, spatial.NewZeroOrientation())
	test.That(t, partAgain.ModelFrame, test.ShouldResemble, part.ModelFrame)

	// fully specified part
	part = &FrameSystemPart{
		Name:        "test",
		FrameConfig: &Frame{Parent: "world", Translation: r3.Vector{1, 2, 3}, Orientation: spatial.NewZeroOrientation()},
		ModelFrame:  model,
	}
	result, err = part.ToProtobuf()
	test.That(t, err, test.ShouldBeNil)
	pose = &commonpb.Pose{X: 1, Y: 2, Z: 3, OZ: 1, Theta: 0}
	exp = &robotpb.FrameSystemConfig{
		Name: "test",
		PoseInParentFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose:           pose,
		},
		ModelJson: jsonData,
	}
	test.That(t, result.Name, test.ShouldEqual, exp.Name)
	test.That(t, result.PoseInParentFrame, test.ShouldResemble, exp.PoseInParentFrame)
	test.That(t, result.ModelJson, test.ShouldNotBeNil)
	// return to FrameSystemPart
	partAgain, err = ProtobufToFrameSystemPart(result)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, partAgain.Name, test.ShouldEqual, part.Name)
	test.That(t, partAgain.FrameConfig.Parent, test.ShouldEqual, part.FrameConfig.Parent)
	test.That(t, partAgain.FrameConfig.Translation, test.ShouldResemble, part.FrameConfig.Translation)
	test.That(t, partAgain.FrameConfig.Orientation, test.ShouldResemble, part.FrameConfig.Orientation)
	test.That(t, partAgain.ModelFrame.Name, test.ShouldEqual, part.ModelFrame.Name)
	test.That(t,
		len(partAgain.ModelFrame.(*referenceframe.SimpleModel).OrdTransforms),
		test.ShouldEqual,
		len(part.ModelFrame.(*referenceframe.SimpleModel).OrdTransforms),
	)
}

func TestFramesFromPart(t *testing.T) {
	logger := golog.NewTestLogger(t)
	jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame.json"))
	test.That(t, err, test.ShouldBeNil)
	model, err := referenceframe.UnmarshalModelJSON(jsonData, "")
	test.That(t, err, test.ShouldBeNil)
	// minimally specified part
	part := &FrameSystemPart{
		Name:        "test",
		FrameConfig: nil,
		ModelFrame:  nil,
	}
	_, _, err = CreateFramesFromPart(part, logger)
	test.That(t, err, test.ShouldBeError, errors.New("config for FrameSystemPart is nil"))

	// slightly specified part
	part = &FrameSystemPart{
		Name:        "test",
		FrameConfig: &Frame{Parent: "world"},
		ModelFrame:  nil,
	}
	modelFrame, originFrame, err := CreateFramesFromPart(part, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, modelFrame, test.ShouldResemble, referenceframe.NewZeroStaticFrame(part.Name))
	test.That(t, originFrame, test.ShouldResemble, referenceframe.NewZeroStaticFrame(part.Name+"_origin"))

	// fully specified part
	part = &FrameSystemPart{
		Name:        "test",
		FrameConfig: &Frame{Parent: "world", Translation: r3.Vector{1, 2, 3}, Orientation: spatial.NewZeroOrientation()},
		ModelFrame:  model,
	}
	modelFrame, originFrame, err = CreateFramesFromPart(part, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, modelFrame.Name(), test.ShouldEqual, part.Name)
	test.That(t, modelFrame.DoF(), test.ShouldResemble, part.ModelFrame.DoF())
	test.That(t, originFrame.Name(), test.ShouldEqual, part.Name+"_origin")
	test.That(t, originFrame.DoF(), test.ShouldHaveLength, 0)
}

func TestConvertTransformProtobufToFrameSystemPart(t *testing.T) {
	t.Run("fails on missing reference frame name", func(t *testing.T) {
		transform := referenceframe.NewPoseInFrame("parent", spatial.NewZeroPose())
		part, err := PoseInFrameToFrameSystemPart(transform)
		test.That(t, err, test.ShouldBeError, referenceframe.ErrEmptyStringFrameName)
		test.That(t, part, test.ShouldBeNil)
	})
	t.Run("converts to frame system part", func(t *testing.T) {
		testPose := spatial.NewPoseFromOrientation(r3.Vector{X: 1., Y: 2., Z: 3.}, &spatial.R4AA{Theta: math.Pi / 2, RX: 0, RY: 1, RZ: 0})
		transform := referenceframe.NewNamedPoseInFrame("parent", testPose, "child")
		part, err := PoseInFrameToFrameSystemPart(transform)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, part.Name, test.ShouldEqual, transform.Name())
		test.That(t, part.FrameConfig.Parent, test.ShouldEqual, transform.FrameName())
		test.That(t, spatial.R3VectorAlmostEqual(part.FrameConfig.Translation, testPose.Point(), 1e-8), test.ShouldBeTrue)
		test.That(t, spatial.OrientationAlmostEqual(part.FrameConfig.Orientation, testPose.Orientation()), test.ShouldBeTrue)
	})
}
