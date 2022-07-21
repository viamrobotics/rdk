package config

import (
	"io/ioutil"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	robotpb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	spatial "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

func TestFrameModelPart(t *testing.T) {
	jsonData, err := ioutil.ReadFile(rdkutils.ResolveFile("config/data/model_frame.json"))
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
		FrameConfig: &Frame{Parent: "world", Translation: spatial.TranslationConfig{1, 2, 3}, Orientation: spatial.NewZeroOrientation()},
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
	jsonData, err := ioutil.ReadFile(rdkutils.ResolveFile("config/data/model_frame.json"))
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
	modelFrame, offsetFrame, err := CreateFramesFromPart(part, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, modelFrame, test.ShouldResemble, referenceframe.NewZeroStaticFrame(part.Name))
	test.That(t, offsetFrame, test.ShouldResemble, referenceframe.NewZeroStaticFrame(part.Name+"_offset"))

	// fully specified part
	part = &FrameSystemPart{
		Name:        "test",
		FrameConfig: &Frame{Parent: "world", Translation: spatial.TranslationConfig{1, 2, 3}, Orientation: spatial.NewZeroOrientation()},
		ModelFrame:  model,
	}
	modelFrame, offsetFrame, err = CreateFramesFromPart(part, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, modelFrame.Name(), test.ShouldEqual, part.Name)
	test.That(t, modelFrame.DoF(), test.ShouldResemble, part.ModelFrame.DoF())
	test.That(t, offsetFrame.Name(), test.ShouldEqual, part.Name+"_offset")
	test.That(t, offsetFrame.DoF(), test.ShouldHaveLength, 0)
}

func TestConvertTransformProtobufToFrameSystemPart(t *testing.T) {
	zeroPose := spatial.PoseToProtobuf(spatial.NewZeroPose())
	t.Run("fails on missing reference frame name", func(t *testing.T) {
		transform := &commonpb.Transform{
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "parent",
				Pose:           zeroPose,
			},
		}
		part, err := ConvertTransformProtobufToFrameSystemPart(transform)
		test.That(t, err, test.ShouldBeError, NewMissingReferenceFrameError(&commonpb.Transform{}))
		test.That(t, part, test.ShouldBeNil)
	})
	t.Run("fails on missing reference frame name", func(t *testing.T) {
		transform := &commonpb.Transform{
			ReferenceFrame: "child",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				Pose: zeroPose,
			},
		}
		part, err := ConvertTransformProtobufToFrameSystemPart(transform)
		test.That(t, err, test.ShouldBeError, NewMissingReferenceFrameError(&commonpb.PoseInFrame{}))
		test.That(t, part, test.ShouldBeNil)
	})
	t.Run("converts to frame system part", func(t *testing.T) {
		testPose := spatial.NewPoseFromOrientation(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0, RY: 1, RZ: 0},
		)
		transform := &commonpb.Transform{
			ReferenceFrame: "child",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "parent",
				Pose:           spatial.PoseToProtobuf(testPose),
			},
		}
		transformPOF := transform.GetPoseInObserverFrame()
		posePt := testPose.Point()
		part, err := ConvertTransformProtobufToFrameSystemPart(transform)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, part.Name, test.ShouldEqual, transform.GetReferenceFrame())
		test.That(t, part.FrameConfig.Parent, test.ShouldEqual, transformPOF.GetReferenceFrame())
		partTrans := part.FrameConfig.Translation
		partOrient := part.FrameConfig.Orientation
		test.That(t, partTrans.X, test.ShouldAlmostEqual, posePt.X)
		test.That(t, partTrans.Y, test.ShouldAlmostEqual, posePt.Y)
		test.That(t, partTrans.Z, test.ShouldAlmostEqual, posePt.Z)
		test.That(t, spatial.OrientationAlmostEqual(partOrient, testPose.Orientation()), test.ShouldBeTrue)
	})
}
