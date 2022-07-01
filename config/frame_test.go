package config

import (
	"encoding/json"
	"io/ioutil"
	"math"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

func TestFrame(t *testing.T) {
	file, err := os.Open("data/frame.json")
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(file.Close)

	data, err := ioutil.ReadAll(file)
	test.That(t, err, test.ShouldBeNil)
	// Parse into map of tests
	var testMap map[string]json.RawMessage
	err = json.Unmarshal(data, &testMap)
	test.That(t, err, test.ShouldBeNil)

	frame := Frame{}
	err = json.Unmarshal(testMap["test"], &frame)
	test.That(t, err, test.ShouldBeNil)
	exp := Frame{
		Parent:      "world",
		Translation: spatial.TranslationConfig{1, 2, 3},
		Orientation: &spatial.OrientationVectorDegrees{Theta: 85, OZ: 1},
	}
	test.That(t, frame, test.ShouldResemble, exp)

	pose := frame.Pose()
	expPose := spatial.NewPoseFromOrientation(r3.Vector{1, 2, 3}, exp.Orientation)
	test.That(t, pose, test.ShouldResemble, expPose)

	staticFrame, err := frame.StaticFrame("test")
	test.That(t, err, test.ShouldBeNil)
	expStaticFrame, err := referenceframe.NewStaticFrame("test", expPose)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, staticFrame, test.ShouldResemble, expStaticFrame)
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
	poseStart := spatial.NewZeroPose()                         // PoV from frame 2
	poseEnd := spatial.NewPoseFromPoint(r3.Vector{-9, -2, -3}) // PoV from frame 4
	transformPoint, err := fs1.Transform(blankPos, referenceframe.NewPoseInFrame("frame2", poseStart), "frame4")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint.(*referenceframe.PoseInFrame).Pose(), poseEnd), test.ShouldBeTrue)

	// reset fs1 framesystem to original
	fs1 = referenceframe.NewEmptySimpleFrameSystem("test1")
	err = fs1.AddFrame(frame1, fs1.World())
	test.That(t, err, test.ShouldBeNil)
	err = fs1.AddFrame(frame2, fs1.GetFrame("frame1"))
	test.That(t, err, test.ShouldBeNil)

	// merge to fs1 with an offset and rotation
	offsetConfig := &Frame{
		Parent: "frame1", Translation: spatial.TranslationConfig{1, 2, 3},
		Orientation: &spatial.R4AA{Theta: math.Pi / 2, RZ: 1.},
	}
	err = MergeFrameSystems(fs1, fs2, offsetConfig)
	test.That(t, err, test.ShouldBeNil)
	// the frame of test2_world is rotated around z by 90 degrees, then displaced by (1,2,3) in the frame of frame1,
	// so the origin of frame1 from the perspective of test2_frame should be (-2, 1, -3)
	poseStart = spatial.NewZeroPose()                        // PoV from frame 1
	poseEnd = spatial.NewPoseFromPoint(r3.Vector{-2, 1, -3}) // PoV from the world of test2
	transformPoint, err = fs1.Transform(blankPos, referenceframe.NewPoseInFrame("frame1", poseStart), "test2_world")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint.(*referenceframe.PoseInFrame).Pose(), poseEnd), test.ShouldBeTrue)

	// frame frame 2 to frame 4
	poseStart = spatial.NewZeroPose()                        // PoV from frame 2
	poseEnd = spatial.NewPoseFromPoint(r3.Vector{-6, -6, 0}) // PoV from frame 4
	transformPoint, err = fs1.Transform(blankPos, referenceframe.NewPoseInFrame("frame2", poseStart), "frame4")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatial.PoseAlmostCoincident(transformPoint.(*referenceframe.PoseInFrame).Pose(), poseEnd), test.ShouldBeTrue)
}
