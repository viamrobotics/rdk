package config

import (
	"io/ioutil"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

func TestFrameModelPart(t *testing.T) {
	jsonData, err := ioutil.ReadFile(rdkutils.ResolveFile("config/data/model_frame.json"))
	test.That(t, err, test.ShouldBeNil)
	model, err := referenceframe.ParseJSON(jsonData, "")
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
	pose := &pb.Pose{OZ: 1, Theta: 0} // zero pose
	exp := &pb.FrameSystemConfig{Name: "test", FrameConfig: &pb.FrameConfig{Parent: "world", Pose: pose}}
	test.That(t, result.Name, test.ShouldEqual, exp.Name)
	test.That(t, result.FrameConfig, test.ShouldResemble, exp.FrameConfig)
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
		FrameConfig: &Frame{Parent: "world", Translation: spatial.Translation{1, 2, 3}, Orientation: spatial.NewZeroOrientation()},
		ModelFrame:  model,
	}
	result, err = part.ToProtobuf()
	test.That(t, err, test.ShouldBeNil)
	pose = &pb.Pose{X: 1, Y: 2, Z: 3, OZ: 1, Theta: 0}
	exp = &pb.FrameSystemConfig{Name: "test", FrameConfig: &pb.FrameConfig{Parent: "world", Pose: pose}, ModelJson: jsonData}
	test.That(t, result.Name, test.ShouldEqual, exp.Name)
	test.That(t, result.FrameConfig, test.ShouldResemble, exp.FrameConfig)
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
	model, err := referenceframe.ParseJSON(jsonData, "")
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
		FrameConfig: &Frame{Parent: "world", Translation: spatial.Translation{1, 2, 3}, Orientation: spatial.NewZeroOrientation()},
		ModelFrame:  model,
	}
	modelFrame, offsetFrame, err = CreateFramesFromPart(part, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, modelFrame.Name(), test.ShouldEqual, part.Name)
	test.That(t, modelFrame.DoF(), test.ShouldResemble, part.ModelFrame.DoF())
	test.That(t, offsetFrame.Name(), test.ShouldEqual, part.Name+"_offset")
	test.That(t, offsetFrame.DoF(), test.ShouldHaveLength, 0)
}
