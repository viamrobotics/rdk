package referenceframe

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"

	"github.com/edaniels/golog"
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

func TestFrameSystemToPCD(t *testing.T) {
	//nolint:dupl
	checkAgainst0 := []r3.Vector{
		{-3.500000000000000000000000, -4, -4},
		{-4.500000000000000000000000, -4, -4},
		{-3.500000000000000000000000, -4, -3.500000000000000000000000},
		{-3.500000000000000000000000, -4, -4.500000000000000000000000},
		{-4.500000000000000000000000, -4, -3.500000000000000000000000},
		{-4.500000000000000000000000, -4, -4.500000000000000000000000},
		{-4, -3.500000000000000000000000, -4},
		{-4, -4.500000000000000000000000, -4},
		{-4, -3.500000000000000000000000, -3.500000000000000000000000},
		{-4, -3.500000000000000000000000, -4.500000000000000000000000},
		{-4, -4.500000000000000000000000, -3.500000000000000000000000},
		{-4, -4.500000000000000000000000, -4.500000000000000000000000},
		{-3.500000000000000000000000, -3.500000000000000000000000, -4},
		{-3.500000000000000000000000, -4.500000000000000000000000, -4},
		{-4.500000000000000000000000, -3.500000000000000000000000, -4},
		{-4.500000000000000000000000, -4.500000000000000000000000, -4},
		{-3.500000000000000000000000, -3.500000000000000000000000, -3.500000000000000000000000},
		{-3.500000000000000000000000, -3.500000000000000000000000, -4.500000000000000000000000},
		{-3.500000000000000000000000, -4.500000000000000000000000, -3.500000000000000000000000},
		{-3.500000000000000000000000, -4.500000000000000000000000, -4.500000000000000000000000},
		{-4.500000000000000000000000, -3.500000000000000000000000, -3.500000000000000000000000},
		{-4.500000000000000000000000, -3.500000000000000000000000, -4.500000000000000000000000},
		{-4.500000000000000000000000, -4.500000000000000000000000, -4.500000000000000000000000},
		{-4.500000000000000000000000, -4.500000000000000000000000, -3.500000000000000000000000},
		{-4, -4, -3.500000000000000000000000},
		{-4, -4, -4.500000000000000000000000},
	}
	//nolint:dupl
	checkAgainst1 := []r3.Vector{
		{-1.500000000000000000000000, -2, -2},
		{-2.500000000000000000000000, -2, -2},
		{-1.500000000000000000000000, -2, -1.500000000000000000000000},
		{-1.500000000000000000000000, -2, -2.500000000000000000000000},
		{-2.500000000000000000000000, -2, -1.500000000000000000000000},
		{-2.500000000000000000000000, -2, -2.500000000000000000000000},
		{-2, -1.500000000000000000000000, -2},
		{-2, -2.500000000000000000000000, -2},
		{-2, -1.500000000000000000000000, -1.500000000000000000000000},
		{-2, -1.500000000000000000000000, -2.500000000000000000000000},
		{-2, -2.500000000000000000000000, -1.500000000000000000000000},
		{-2, -2.500000000000000000000000, -2.500000000000000000000000},
		{-1.500000000000000000000000, -1.500000000000000000000000, -2},
		{-1.500000000000000000000000, -2.500000000000000000000000, -2},
		{-2.500000000000000000000000, -1.500000000000000000000000, -2},
		{-2.500000000000000000000000, -2.500000000000000000000000, -2},
		{-1.500000000000000000000000, -1.500000000000000000000000, -1.500000000000000000000000},
		{-1.500000000000000000000000, -1.500000000000000000000000, -2.500000000000000000000000},
		{-1.500000000000000000000000, -2.500000000000000000000000, -1.500000000000000000000000},
		{-1.500000000000000000000000, -2.500000000000000000000000, -2.500000000000000000000000},
		{-2.500000000000000000000000, -1.500000000000000000000000, -1.500000000000000000000000},
		{-2.500000000000000000000000, -1.500000000000000000000000, -2.500000000000000000000000},
		{-2.500000000000000000000000, -2.500000000000000000000000, -2.500000000000000000000000},
		{-2.500000000000000000000000, -2.500000000000000000000000, -1.500000000000000000000000},
		{-2, -2, -1.500000000000000000000000},
		{-2, -2, -2.500000000000000000000000},
	}

	t.Run("displaced box with another box as its child", func(t *testing.T) {
		fs := NewEmptyFrameSystem("test")
		logger := golog.NewTestLogger(t)
		// ------
		name0 := "frame0"
		pose0 := spatial.NewPoseFromPoint(r3.Vector{-4, -4, -4})
		dims0 := r3.Vector{1, 1, 1}
		geomCreator0, err := spatial.NewBox(pose0, dims0, "box0")
		test.That(t, err, test.ShouldBeNil)
		frame0, err := NewStaticFrameWithGeometry(name0, pose0, geomCreator0)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(frame0, fs.World())
		// -----
		name1 := "frame1"
		pose1 := spatial.NewPoseFromPoint(r3.Vector{2, 2, 2})
		dims1 := r3.Vector{1, 1, 1}
		geomCreator1, err := spatial.NewBox(pose1, dims1, "box1")
		test.That(t, err, test.ShouldBeNil)
		frame1, err := NewStaticFrameWithGeometry(name1, pose1, geomCreator1)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(frame1, frame0)
		// -----
		inputs := StartPositions(fs)
		outMap, err := FrameSystemToPCD(fs, inputs, logger)
		test.That(t, err, test.ShouldBeNil)

		for i, v := range outMap["frame0"] {
			test.That(t, spatial.R3VectorAlmostEqual(v, checkAgainst0[i], 1e-2), test.ShouldBeTrue)
		}
		for i, v := range outMap["frame1"] {
			test.That(t, spatial.R3VectorAlmostEqual(v, checkAgainst1[i], 1e-2), test.ShouldBeTrue)
		}
	})
	t.Run("displaced box with another box as its child with nil inputs", func(t *testing.T) {
		fs := NewEmptyFrameSystem("test")
		logger := golog.NewTestLogger(t)
		// ------
		name0 := "frame0"
		pose0 := spatial.NewPoseFromPoint(r3.Vector{-4, -4, -4})
		dims0 := r3.Vector{1, 1, 1}
		geomCreator0, err := spatial.NewBox(pose0, dims0, "box0")
		test.That(t, err, test.ShouldBeNil)
		frame0, err := NewStaticFrameWithGeometry(name0, pose0, geomCreator0)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(frame0, fs.World())
		// -----
		name1 := "frame1"
		pose1 := spatial.NewPoseFromPoint(r3.Vector{2, 2, 2})
		dims1 := r3.Vector{1, 1, 1}
		geomCreator1, err := spatial.NewBox(pose1, dims1, "box1")
		test.That(t, err, test.ShouldBeNil)
		frame1, err := NewStaticFrameWithGeometry(name1, pose1, geomCreator1)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(frame1, frame0)
		// -----
		outMap, err := FrameSystemToPCD(fs, nil, logger)
		test.That(t, err, test.ShouldBeNil)

		for i, v := range outMap["frame0"] {
			test.That(t, spatial.R3VectorAlmostEqual(v, checkAgainst0[i], 1e-2), test.ShouldBeTrue)
		}
		for i, v := range outMap["frame1"] {
			test.That(t, spatial.R3VectorAlmostEqual(v, checkAgainst1[i], 1e-2), test.ShouldBeTrue)
		}
	})

	t.Run("incorrectly defined frame system, i.e. with nil parent for frame0", func(t *testing.T) {
		fs := NewEmptyFrameSystem("test")
		logger := golog.NewTestLogger(t)
		// ------
		name := "frame"
		pose := spatial.NewPoseFromPoint(r3.Vector{-4, -4, -4})
		dims := r3.Vector{1, 1, 1}
		geomCreator, err := spatial.NewBox(pose, dims, "box")
		test.That(t, err, test.ShouldBeNil)
		frame, err := NewStaticFrameWithGeometry(name, pose, geomCreator)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(frame, nil)
		// -----
		inputs := StartPositions(fs)
		outMap, err := FrameSystemToPCD(fs, inputs, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, outMap, test.ShouldBeEmpty)
	})

	t.Run("correct frame system with nil input and DOF = 0", func(t *testing.T) {
		fs := NewEmptyFrameSystem("test")
		logger := golog.NewTestLogger(t)
		// ------
		name := "frame"
		pose := spatial.NewPoseFromPoint(r3.Vector{-4, -4, -4})
		dims := r3.Vector{1, 1, 1}
		geomCreator, err := spatial.NewBox(pose, dims, "box")
		test.That(t, err, test.ShouldBeNil)
		frame, err := NewStaticFrameWithGeometry(name, pose, geomCreator)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(frame, fs.World())
		// -----
		outMap, err := FrameSystemToPCD(fs, nil, logger)
		test.That(t, err, test.ShouldBeNil)
		for i, v := range outMap["frame"] {
			test.That(t, spatial.R3VectorAlmostEqual(v, checkAgainst0[i], 1e-2), test.ShouldBeTrue)
		}
	})

	t.Run("correct frame system with nil input and DOF != 0", func(t *testing.T) {
		fs := NewEmptyFrameSystem("test")
		logger := golog.NewTestLogger(t)
		jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame_geoms.json"))
		test.That(t, err, test.ShouldBeNil)
		model, err := UnmarshalModelJSON(jsonData, "")
		test.That(t, err, test.ShouldBeNil)
		orientConf, err := spatial.NewOrientationConfig(spatial.NewZeroOrientation())
		test.That(t, err, test.ShouldBeNil)
		lc := &LinkConfig{
			ID:          "arm",
			Parent:      "world",
			Translation: r3.Vector{1, 2, 3},
			Orientation: orientConf,
		}
		lif, err := lc.ParseConfig()
		test.That(t, err, test.ShouldBeNil)
		// fully specified part
		part := &FrameSystemPart{
			FrameConfig: lif,
			ModelFrame:  model,
		}
		armFrame, _, err := createFramesFromPart(part)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(armFrame, fs.World())
		// -----
		outMap, err := FrameSystemToPCD(fs, nil, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, outMap, test.ShouldBeEmpty)
	})
	t.Run("correct frame system with a parent child relationship, with nil input and DOF != 0", func(t *testing.T) {
		fs := NewEmptyFrameSystem("test")
		logger := golog.NewTestLogger(t)
		jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame_geoms.json"))
		test.That(t, err, test.ShouldBeNil)
		model, err := UnmarshalModelJSON(jsonData, "")
		test.That(t, err, test.ShouldBeNil)
		orientConf, err := spatial.NewOrientationConfig(spatial.NewZeroOrientation())
		test.That(t, err, test.ShouldBeNil)
		lc := &LinkConfig{
			ID:          "arm",
			Parent:      "world",
			Translation: r3.Vector{1, 2, 3},
			Orientation: orientConf,
		}
		lif, err := lc.ParseConfig()
		test.That(t, err, test.ShouldBeNil)
		// fully specified part
		part := &FrameSystemPart{
			FrameConfig: lif,
			ModelFrame:  model,
		}
		armFrame, _, err := createFramesFromPart(part)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(armFrame, fs.World())
		// -----
		blockName := "block"
		blockPose := spatial.NewPoseFromPoint(r3.Vector{2, 2, 2})
		blockdims := r3.Vector{10, 10, 10}
		blockGeomCreator, err := spatial.NewBox(blockPose, blockdims, "box")
		test.That(t, err, test.ShouldBeNil)
		blockFrame, err := NewStaticFrameWithGeometry(blockName, blockPose, blockGeomCreator)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(blockFrame, armFrame)
		// -----
		name := "frame"
		pose := spatial.NewPoseFromPoint(r3.Vector{-4, -4, -4})
		dims := r3.Vector{1, 1, 1}
		geomCreator, err := spatial.NewBox(pose, dims, "box")
		test.That(t, err, test.ShouldBeNil)
		frame, err := NewStaticFrameWithGeometry(name, pose, geomCreator)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(frame, fs.World())
		// -----
		outMap, err := FrameSystemToPCD(fs, nil, logger)
		test.That(t, err, test.ShouldBeNil)
		for i, v := range outMap["frame"] {
			test.That(t, spatial.R3VectorAlmostEqual(v, checkAgainst0[i], 1e-2), test.ShouldBeTrue)
		}
	})

	t.Run("arm with a block attached to the end effector", func(t *testing.T) {
		fs := NewEmptyFrameSystem("test")
		logger := golog.NewTestLogger(t)
		jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame_geoms.json"))
		test.That(t, err, test.ShouldBeNil)
		model, err := UnmarshalModelJSON(jsonData, "")
		test.That(t, err, test.ShouldBeNil)
		orientConf, err := spatial.NewOrientationConfig(spatial.NewZeroOrientation())
		test.That(t, err, test.ShouldBeNil)
		lc := &LinkConfig{
			ID:          "arm",
			Parent:      "world",
			Translation: r3.Vector{1, 2, 3},
			Orientation: orientConf,
		}
		lif, err := lc.ParseConfig()
		test.That(t, err, test.ShouldBeNil)
		// fully specified part
		part := &FrameSystemPart{
			FrameConfig: lif,
			ModelFrame:  model,
		}
		armFrame, _, err := createFramesFromPart(part)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(armFrame, fs.World())
		blockName := "block"
		blockPose := spatial.NewPoseFromPoint(r3.Vector{2, 2, 2})
		blockdims := r3.Vector{10, 10, 10}
		blockGeomCreator, err := spatial.NewBox(blockPose, blockdims, "box1")
		test.That(t, err, test.ShouldBeNil)
		blockFrame, err := NewStaticFrameWithGeometry(blockName, blockPose, blockGeomCreator)
		test.That(t, err, test.ShouldBeNil)
		fs.AddFrame(blockFrame, armFrame)
		// -----
		inputs := StartPositions(fs)
		outMap, err := FrameSystemToPCD(fs, inputs, logger)
		test.That(t, err, test.ShouldBeNil)

		// 0. get output values
		total := outMap["test"]
		total = append(total, outMap["block"]...)
		// 1. round all values
		for i := range total {
			total[i].X = math.Round(total[i].X*10) / 10
			total[i].Y = math.Round(total[i].Y*10) / 10
			total[i].Z = math.Round(total[i].Z*10) / 10
		}
		// 2. sort
		sort.SliceStable(total, func(i, j int) bool {
			return total[i].X < total[j].X
		})
		// 3. encode the value
		var network bytes.Buffer        // Stand-in for a network connection
		enc := gob.NewEncoder(&network) // Will write to network.
		err = enc.Encode(total)
		test.That(t, err, test.ShouldBeNil)
		// 4. Hash the bytes
		asBytes := md5.Sum(network.Bytes())
		checkAgainst := [16]uint8{242, 99, 115, 21, 213, 207, 247, 66, 243, 191, 235, 225, 126, 164, 176, 42}
		test.That(t, asBytes, test.ShouldEqual, checkAgainst)
	})
}

func TestReplaceFrame(t *testing.T) {
	fs := NewEmptyFrameSystem("test")
	pose := spatial.NewZeroPose()
	dims := r3.Vector{1, 1, 1}
	box, err := spatial.NewBox(pose, dims, "box")
	test.That(t, err, test.ShouldBeNil)

	// ------ replaceMe frame
	replaceMe, err := NewStaticFrameWithGeometry("replaceMe", pose, box)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(replaceMe, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// ------ frame1
	frame1, err := NewStaticFrameWithGeometry("frame1", pose, box)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame1, replaceMe)
	test.That(t, err, test.ShouldBeNil)

	// ------ frame2
	frame2, err := NewStaticFrameWithGeometry("frame2", pose, box)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame2, frame1)
	test.That(t, err, test.ShouldBeNil)

	// ------ frame3
	frame3, err := NewStaticFrameWithGeometry("frame3", pose, box)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(frame3, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// ------ replaceWith frame
	replaceWith, err := NewStaticFrameWithGeometry("replaceWith", pose, box)
	test.That(t, err, test.ShouldBeNil)

	err = fs.ReplaceFrame(fs, replaceMe, replaceWith)
	test.That(t, err, test.ShouldBeNil)

	// make sure replaceMe is gone
	test.That(t, fs.Frame(replaceMe.Name()), test.ShouldBeNil)

	// make sure parentage is transferred successfully
	f, err := fs.Parent(replaceWith)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f.Name(), test.ShouldEqual, fs.World().Name())

	// make sure parentage is preserved
	f, err = fs.Parent(frame1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f.Name(), test.ShouldEqual, replaceWith.Name())

	f, err = fs.Parent(frame2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f.Name(), test.ShouldEqual, frame1.Name())

	f, err = fs.Parent(frame3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f.Name(), test.ShouldEqual, fs.World().Name())
}
