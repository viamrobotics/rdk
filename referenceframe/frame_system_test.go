package referenceframe

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/emre/golist"
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/spatialmath"
	spatial "go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

func TestFrameModelPart(t *testing.T) {
	jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame.json"))
	test.That(t, err, test.ShouldBeNil)
	var modelJSONMap map[string]interface{}
	json.Unmarshal(jsonData, &modelJSONMap)
	kinematics, err := protoutils.StructToStructPb(modelJSONMap)
	test.That(t, err, test.ShouldBeNil)
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
		Kinematics: kinematics,
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
	logger := golog.NewTestLogger(t)
	jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame_geoms.json"))
	test.That(t, err, test.ShouldBeNil)
	model, err := UnmarshalModelJSON(jsonData, "")
	test.That(t, err, test.ShouldBeNil)
	// minimally specified part
	part := &FrameSystemPart{
		FrameConfig: &LinkInFrame{PoseInFrame: &PoseInFrame{name: "test"}},
		ModelFrame:  nil,
	}
	_, _, err = CreateFramesFromPart(part, logger)
	test.That(t, err, test.ShouldBeNil)

	// slightly specified part
	part = &FrameSystemPart{
		FrameConfig: &LinkInFrame{PoseInFrame: &PoseInFrame{name: "test", parent: "world"}},
		ModelFrame:  nil,
	}
	modelFrame, originFrame, err := CreateFramesFromPart(part, logger)
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
	modelFrame, originFrame, err = CreateFramesFromPart(part, logger)
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
	modelFrame, originFrame, err = CreateFramesFromPart(part, logger)
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
	modelFrame, originFrame, err = CreateFramesFromPart(part, logger)
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
	fs := NewEmptySimpleFrameSystem("test")
	// ------
	name0 := "frame0"
	pose0 := spatial.NewPoseFromPoint(r3.Vector{-4, -4, -4})
	dims0 := r3.Vector{2, 2, 2}
	// offset0 := spatial.NewPoseFromPoint(r3.Vector{-20, -20, -20})
	geomCreator0, _ := spatialmath.NewBox(pose0, dims0, "box0")
	// geomCreator0, _ := spatial.NewBoxCreator(dims0, offset0, "box0")
	frame0, _ := NewStaticFrameWithGeometry(name0, pose0, geomCreator0)
	fs.AddFrame(frame0, fs.World())
	// -----
	name1 := "frame1"
	pose1 := spatial.NewPoseFromPoint(r3.Vector{2, 2, 2})
	dims1 := r3.Vector{2, 2, 2}
	// offset1 := spatial.NewPoseFromPoint(r3.Vector{10, 10, 10})
	geomCreator1, _ := spatial.NewBox(pose1, dims1, "box1")
	// geomCreator1, _ := spatial.NewBoxCreator(dims1, offset1, "box1")
	frame1, _ := NewStaticFrameWithGeometry(name1, pose1, geomCreator1)
	fs.AddFrame(frame1, frame0)

	// ---------------------------------------------
	// logger := golog.NewTestLogger(t)
	// jsonData, err := os.ReadFile(rdkutils.ResolveFile("config/data/model_frame_geoms.json"))
	// test.That(t, err, test.ShouldBeNil)
	// model, err := UnmarshalModelJSON(jsonData, "")
	// orientConf, err := spatial.NewOrientationConfig(spatial.NewZeroOrientation())
	// test.That(t, err, test.ShouldBeNil)

	// lc := &LinkConfig{
	// 	ID:          "test",
	// 	Parent:      "world",
	// 	Translation: r3.Vector{1, 2, 3},
	// 	Orientation: orientConf,
	// }
	// lif, err := lc.ParseConfig()
	// test.That(t, err, test.ShouldBeNil)
	// // fully specified part
	// part := &FrameSystemPart{
	// 	FrameConfig: lif,
	// 	ModelFrame:  model,
	// }
	// frameA1, _, _ := CreateFramesFromPart(part, logger)
	// fs.AddFrame(frameA1, fs.World())

	inputs := make(map[string][]Input)
	// inputs["test"] = make([]Input, 6)
	// inputs["test_origin"] = make([]Input, 0)
	inputs["frame0"] = make([]Input, 0)
	inputs["frame1"] = make([]Input, 0)

	outMap, _ := FrameSystemToPCD(fs, inputs)
	// check1 := outMap["test"]
	// check2 := outMap["test_origin"]
	check1 := outMap["frame0"]
	check2 := outMap["frame1"]
	// check3 := outMap["test"]

	// colorss1 := color.NRGBA{255, 0, 0, 255}
	// colorss2 := color.NRGBA{0, 255, 0, 255}
	// // colorss3 := color.NRGBA{0, 0, 255, 255}
	// var cluster []pointcloud.PointCloud
	// pc1, _ := pointcloud.VectorsToPointCloud(check1, colorss1)
	// pc2, _ := pointcloud.VectorsToPointCloud(check2, colorss2)
	// // pc3, _ := pointcloud.VectorsToPointCloud(check3, colorss3)
	// cluster = append(cluster, pc1)
	// cluster = append(cluster, pc2)
	// // cluster = append(cluster, pc3)
	// merged, _ := pointcloud.MergePointCloudsWithColor(cluster)
	// // write to .PCD file
	// fileName := "wrld.pcd"
	// file, _ := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	// os.Truncate("wrld.pcd", 0)
	// pointcloud.ToPCD(merged, file, 0)

	lastList := golist.New()
	for i := 0; i < len(check1); i++ {
		pointsList := golist.New()
		pointsList.Append(check1[i].X)
		pointsList.Append(check1[i].Y)
		pointsList.Append(check1[i].Z)
		lastList.Append(pointsList)
	}
	f, _ := os.Create("/Users/nick/Desktop/play/wrld1.txt")
	f.WriteString(lastList.String())
	f.Close()

	lastList = golist.New()
	for i := 0; i < len(check2); i++ {
		pointsList := golist.New()
		pointsList.Append(check2[i].X)
		pointsList.Append(check2[i].Y)
		pointsList.Append(check2[i].Z)
		lastList.Append(pointsList)
	}
	f, _ = os.Create("/Users/nick/Desktop/play/wrld2.txt")
	f.WriteString(lastList.String())
	f.Close()
}
