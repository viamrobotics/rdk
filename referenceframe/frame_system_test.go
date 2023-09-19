package referenceframe

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	spatial "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

func TestFrameModelPart(t *testing.T) {
	jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame.json"))
	test.That(t, err, test.ShouldBeNil)
	var modelJSONMap map[string]interface{}
	json.Unmarshal(jsonData, &modelJSONMap)
	model, err := UnmarshalModelJSON(jsonData, "")
	test.That(t, err, test.ShouldBeNil)

	// minimally specified part
	part := &FrameSystemPart{
		FrameConfig: &LinkInFrame{PoseInFrame: &PoseInFrame{name: "test"}},
		ModelFrame:  nil,
	}
	_, err = part.ToProtobuf()
	test.That(t, err, test.ShouldBeNil)

	// slightly specified part
	part = &FrameSystemPart{
		FrameConfig: &LinkInFrame{PoseInFrame: &PoseInFrame{name: "test", parent: "world"}},
		ModelFrame:  nil,
	}
	result, err := part.ToProtobuf()
	test.That(t, err, test.ShouldBeNil)
	pose := &commonpb.Pose{} // zero pose
	exp := &robotpb.FrameSystemConfig{
		Frame: &commonpb.Transform{
			ReferenceFrame: "test",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "world",
				Pose:           pose,
			},
		},
	}
	test.That(t, result.Frame.ReferenceFrame, test.ShouldEqual, exp.Frame.ReferenceFrame)
	test.That(t, result.Frame.PoseInObserverFrame, test.ShouldResemble, exp.Frame.PoseInObserverFrame)
	// exp.Kinematics is nil, but the struct in the struct PB
	expKin, err := protoutils.StructToStructPb(exp.Kinematics)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result.Kinematics, test.ShouldResemble, expKin)
	// return to FrameSystemPart
	partAgain, err := ProtobufToFrameSystemPart(result)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, partAgain.FrameConfig.name, test.ShouldEqual, part.FrameConfig.name)
	test.That(t, partAgain.FrameConfig.parent, test.ShouldEqual, part.FrameConfig.parent)
	test.That(t, partAgain.FrameConfig.pose, test.ShouldResemble, spatial.NewZeroPose())
	// nil orientations become specified as zero orientations
	test.That(t, partAgain.FrameConfig.pose.Orientation(), test.ShouldResemble, spatial.NewZeroOrientation())
	test.That(t, partAgain.ModelFrame, test.ShouldResemble, part.ModelFrame)

	orientConf, err := spatial.NewOrientationConfig(spatial.NewZeroOrientation())
	test.That(t, err, test.ShouldBeNil)

	lc := &LinkConfig{
		ID:          "test",
		Parent:      "world",
		Translation: r3.Vector{1, 2, 3},
		Orientation: orientConf,
	}
	lif, err := lc.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	// fully specified part
	part = &FrameSystemPart{
		FrameConfig: lif,
		ModelFrame:  model,
	}
	result, err = part.ToProtobuf()
	test.That(t, err, test.ShouldBeNil)
	pose = &commonpb.Pose{X: 1, Y: 2, Z: 3, OZ: 1, Theta: 0}
	exp = &robotpb.FrameSystemConfig{
		Frame: &commonpb.Transform{
			ReferenceFrame: "test",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "world",
				Pose:           pose,
			},
		},
	}
	test.That(t, result.Frame.ReferenceFrame, test.ShouldEqual, exp.Frame.ReferenceFrame)
	test.That(t, result.Frame.PoseInObserverFrame, test.ShouldResemble, exp.Frame.PoseInObserverFrame)
	test.That(t, result.Kinematics, test.ShouldNotBeNil)
	// return to FrameSystemPart
	partAgain, err = ProtobufToFrameSystemPart(result)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, partAgain.FrameConfig.name, test.ShouldEqual, part.FrameConfig.name)
	test.That(t, partAgain.FrameConfig.parent, test.ShouldEqual, part.FrameConfig.parent)
	test.That(t, partAgain.FrameConfig.pose, test.ShouldResemble, part.FrameConfig.pose)
	test.That(t, partAgain.ModelFrame.Name, test.ShouldEqual, part.ModelFrame.Name)
	test.That(t,
		len(partAgain.ModelFrame.(*SimpleModel).OrdTransforms),
		test.ShouldEqual,
		len(part.ModelFrame.(*SimpleModel).OrdTransforms),
	)
}

func TestFramesFromPart(t *testing.T) {
	jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame_geoms.json"))
	test.That(t, err, test.ShouldBeNil)
	model, err := UnmarshalModelJSON(jsonData, "")
	test.That(t, err, test.ShouldBeNil)
	// minimally specified part
	part := &FrameSystemPart{
		FrameConfig: &LinkInFrame{PoseInFrame: &PoseInFrame{name: "test"}},
		ModelFrame:  nil,
	}
	_, _, err = createFramesFromPart(part)
	test.That(t, err, test.ShouldBeNil)

	// slightly specified part
	part = &FrameSystemPart{
		FrameConfig: &LinkInFrame{PoseInFrame: &PoseInFrame{name: "test", parent: "world"}},
		ModelFrame:  nil,
	}
	modelFrame, originFrame, err := createFramesFromPart(part)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, modelFrame, test.ShouldResemble, NewZeroStaticFrame(part.FrameConfig.name))
	originTailFrame, ok := NewZeroStaticFrame(part.FrameConfig.name + "_origin").(*staticFrame)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, originFrame, test.ShouldResemble, &tailGeometryStaticFrame{originTailFrame})
	orientConf, err := spatial.NewOrientationConfig(spatial.NewZeroOrientation())
	test.That(t, err, test.ShouldBeNil)

	lc := &LinkConfig{
		ID:          "test",
		Parent:      "world",
		Translation: r3.Vector{1, 2, 3},
		Orientation: orientConf,
	}
	lif, err := lc.ParseConfig()
	test.That(t, err, test.ShouldBeNil)

	// fully specified part
	part = &FrameSystemPart{
		FrameConfig: lif,
		ModelFrame:  model,
	}
	modelFrame, originFrame, err = createFramesFromPart(part)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, modelFrame.Name(), test.ShouldEqual, part.FrameConfig.name)
	test.That(t, modelFrame.DoF(), test.ShouldResemble, part.ModelFrame.DoF())
	test.That(t, originFrame.Name(), test.ShouldEqual, part.FrameConfig.name+"_origin")
	test.That(t, originFrame.DoF(), test.ShouldHaveLength, 0)

	// Test geometries are not overwritten for non-zero DOF frames
	lc = &LinkConfig{
		ID:          "test",
		Parent:      "world",
		Translation: r3.Vector{1, 2, 3},
		Orientation: orientConf,
		Geometry:    &spatial.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
	}
	lif, err = lc.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	part = &FrameSystemPart{
		FrameConfig: lif,
		ModelFrame:  model,
	}
	modelFrame, originFrame, err = createFramesFromPart(part)
	test.That(t, err, test.ShouldBeNil)
	modelGeoms, err := modelFrame.Geometries(make([]Input, len(modelFrame.DoF())))
	test.That(t, err, test.ShouldBeNil)
	originGeoms, err := originFrame.Geometries(make([]Input, len(originFrame.DoF())))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(modelGeoms.Geometries()), test.ShouldBeGreaterThan, 1)
	test.That(t, len(originGeoms.Geometries()), test.ShouldEqual, 1)

	// Test that zero-DOF geometries ARE overwritten
	jsonData, err = os.ReadFile(rdkutils.ResolveFile("config/data/gripper_model.json"))
	test.That(t, err, test.ShouldBeNil)
	model, err = UnmarshalModelJSON(jsonData, "")
	test.That(t, err, test.ShouldBeNil)
	lc = &LinkConfig{
		ID:          "test",
		Parent:      "world",
		Translation: r3.Vector{1, 2, 3},
		Orientation: orientConf,
		Geometry:    &spatial.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
	}
	lif, err = lc.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	part = &FrameSystemPart{
		FrameConfig: lif,
		ModelFrame:  model,
	}
	modelFrame, originFrame, err = createFramesFromPart(part)
	test.That(t, err, test.ShouldBeNil)
	modelFrameGeoms, err := modelFrame.Geometries(make([]Input, len(modelFrame.DoF())))
	test.That(t, err, test.ShouldBeNil)
	modelGeoms, err = model.Geometries(make([]Input, len(model.DoF())))
	test.That(t, err, test.ShouldBeNil)
	originGeoms, err = originFrame.Geometries(make([]Input, len(originFrame.DoF())))
	test.That(t, err, test.ShouldBeNil)

	// Orig model should have 1 geometry, but new model should be wrapped with zero
	test.That(t, len(modelFrameGeoms.Geometries()), test.ShouldEqual, 0)
	test.That(t, len(modelGeoms.Geometries()), test.ShouldEqual, 1)
	test.That(t, len(originGeoms.Geometries()), test.ShouldEqual, 1)
}

func TestConvertTransformProtobufToFrameSystemPart(t *testing.T) {
	t.Run("fails on missing reference frame name", func(t *testing.T) {
		transform := &LinkInFrame{PoseInFrame: NewPoseInFrame("parent", spatial.NewZeroPose())}
		part, err := LinkInFrameToFrameSystemPart(transform)
		test.That(t, err, test.ShouldBeError, ErrEmptyStringFrameName)
		test.That(t, part, test.ShouldBeNil)
	})
	t.Run("converts to frame system part", func(t *testing.T) {
		testPose := spatial.NewPose(r3.Vector{X: 1., Y: 2., Z: 3.}, &spatial.R4AA{Theta: math.Pi / 2, RX: 0, RY: 1, RZ: 0})
		transform := NewLinkInFrame("parent", testPose, "child", nil)
		part, err := LinkInFrameToFrameSystemPart(transform)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, part.FrameConfig.name, test.ShouldEqual, transform.Name())
		test.That(t, part.FrameConfig.parent, test.ShouldEqual, transform.Parent())
		test.That(t, spatial.R3VectorAlmostEqual(part.FrameConfig.pose.Point(), testPose.Point(), 1e-8), test.ShouldBeTrue)
		test.That(t, spatial.OrientationAlmostEqual(part.FrameConfig.pose.Orientation(), testPose.Orientation()), test.ShouldBeTrue)
	})
}

func TestFrameSystemGeometries(t *testing.T) {
	fs := NewEmptyFrameSystem("test")
	dims := r3.Vector{1, 1, 1}

	// add a static frame with a box
	name0 := "frame0"
	pose0 := spatial.NewPoseFromPoint(r3.Vector{-4, -4, -4})
	box0, err := spatial.NewBox(pose0, dims, name0)
	test.That(t, err, test.ShouldBeNil)
	frame0, err := NewStaticFrameWithGeometry(name0, pose0, box0)
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(frame0, fs.World())

	// add a static frame with a box as a child of the first
	name1 := "frame1"
	pose1 := spatial.NewPoseFromPoint(r3.Vector{2, 2, 2})
	box1, err := spatial.NewBox(pose1, dims, name1)
	test.That(t, err, test.ShouldBeNil)
	frame1, err := NewStaticFrameWithGeometry(name1, pose1, box1)
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(frame1, frame0)

	// function to check that boxes are returned and where they are supposed to be
	staticGeometriesOK := func(t *testing.T, geometries map[string]*GeometriesInFrame) {
		t.Helper()
		g0, ok := geometries[name0]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, g0.Parent(), test.ShouldResemble, World)
		test.That(t, g0.Geometries()[0].AlmostEqual(box0), test.ShouldBeTrue)
		g1, ok := geometries[name1]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, g1.Parent(), test.ShouldResemble, World)
		test.That(t, spatial.PoseAlmostCoincident(g1.Geometries()[0].Pose(), spatial.Compose(pose0, pose1)), test.ShouldBeTrue)
	}

	type testCase struct {
		name    string
		inputs  map[string][]Input
		success bool
	}

	// test that boxes are where they should be regardless of input, since neither depend on input to be located
	for _, tc := range []testCase{
		{name: "non-nil inputs, zero DOF", inputs: StartPositions(fs)},
		{name: "nil inputs, zero DOF", inputs: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			geometries, err := FrameSystemGeometries(fs, tc.inputs)
			test.That(t, err, test.ShouldBeNil)
			staticGeometriesOK(t, geometries)
		})
	}

	// add an arm model to the fs
	jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame_geoms.json"))
	test.That(t, err, test.ShouldBeNil)
	model, err := UnmarshalModelJSON(jsonData, "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(model, fs.World())
	eePose, err := model.Transform(make([]Input, len(model.DoF())))
	test.That(t, err, test.ShouldBeNil)

	// add a static frame as a child of the model
	name2 := "block"
	pose2 := spatial.NewPoseFromPoint(r3.Vector{2, 2, 2})
	box2, err := spatial.NewBox(pose2, dims, name2)
	test.That(t, err, test.ShouldBeNil)
	blockFrame, err := NewStaticFrameWithGeometry(name2, pose2, box2)
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(blockFrame, model)

	// function to check that boxes relying on inputs are returned and where they are supposed to be
	dynamicGeometriesOK := func(t *testing.T, geometries map[string]*GeometriesInFrame) {
		t.Helper()
		g0, ok := geometries[model.Name()]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, g0.Parent(), test.ShouldResemble, World)
		test.That(t, len(g0.Geometries()), test.ShouldBeGreaterThan, 0)
		g1, ok := geometries[name2]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, g1.Parent(), test.ShouldResemble, World)
		test.That(t, spatial.PoseAlmostCoincident(g1.Geometries()[0].Pose(), spatial.Compose(eePose, pose1)), test.ShouldBeTrue)
	}

	// test that boxes are where they should be regardless of input, since neither depend on input to be located
	for _, tc := range []testCase{
		{name: "non-nil inputs, non-zero DOF", inputs: StartPositions(fs), success: true},
		{name: "nil inputs, non-zero DOF", inputs: nil, success: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			geometries, err := FrameSystemGeometries(fs, tc.inputs)
			if !tc.success {
				test.That(t, err, test.ShouldNotBeNil)
			} else {
				test.That(t, err, test.ShouldBeNil)
				dynamicGeometriesOK(t, geometries)
			}
			staticGeometriesOK(t, geometries)
		})
	}
}

func TestReplaceFrame(t *testing.T) {
	fs := NewEmptyFrameSystem("test")
	// fill framesystem
	pose := spatial.NewZeroPose()
	box, err := spatial.NewBox(pose, r3.Vector{1, 1, 1}, "box")
	test.That(t, err, test.ShouldBeNil)
	replaceMe, err := NewStaticFrameWithGeometry("replaceMe", pose, box)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(replaceMe, fs.World())
	test.That(t, err, test.ShouldBeNil)

	frame1 := NewZeroStaticFrame("frame1")
	err = fs.AddFrame(frame1, replaceMe)
	test.That(t, err, test.ShouldBeNil)

	frame2 := NewZeroStaticFrame("frame2")
	err = fs.AddFrame(frame2, frame1)
	test.That(t, err, test.ShouldBeNil)

	leafNode, err := NewStaticFrameWithGeometry("leafNode", pose, box)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(leafNode, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// ------ fail with replacing world
	err = fs.ReplaceFrame(fs.World())
	test.That(t, err, test.ShouldNotBeNil)

	// ------ fail replacing with a frame already found in the framesystem
	err = fs.ReplaceFrame(replaceMe)
	test.That(t, err, test.ShouldNotBeNil)

	// ------ fail replacing a frame not found in the framesystem
	ghostFrame := NewZeroStaticFrame("ghost")
	err = fs.ReplaceFrame(ghostFrame)
	test.That(t, err, test.ShouldNotBeNil)

	// ------ replace a non-leaf node
	replaceWith := NewZeroStaticFrame("replaceMe")
	err = fs.ReplaceFrame(replaceWith)
	test.That(t, err, test.ShouldBeNil)

	// ------ replace a leaf node
	newLeafNode := NewZeroStaticFrame("leafNode")
	err = fs.ReplaceFrame(newLeafNode)
	test.That(t, err, test.ShouldBeNil)

	// make sure replaceMe and leafNode are gone
	test.That(t, fs.Frame(replaceWith.Name()), test.ShouldNotResemble, replaceMe)
	test.That(t, fs.Frame(newLeafNode.Name()), test.ShouldNotResemble, leafNode)

	// make sure parentage is transferred successfully
	f, err := fs.Parent(replaceWith)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f, test.ShouldResemble, fs.World())

	// make sure parentage is preserved
	f, err = fs.Parent(frame1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f, test.ShouldResemble, replaceWith)

	f, err = fs.Parent(frame2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f, test.ShouldResemble, frame1)

	f, err = fs.Parent(newLeafNode)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f, test.ShouldResemble, fs.World())
}
