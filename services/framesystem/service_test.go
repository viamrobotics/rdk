package framesystem_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	robotimpl "go.viam.com/rdk/robot/impl"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/services/framesystem"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

var blankPos map[string][]referenceframe.Input

func TestFrameSystemFromConfig(t *testing.T) {
	// use robot/impl/data/fake.json as config input
	emptyIn := []referenceframe.Input{}
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), rdkutils.ResolveFile("robot/impl/data/fake.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer r.Close(context.Background())

	// use fake registrations to have a FrameSystem return
	testPose := spatialmath.NewPoseFromAxisAngle(
		r3.Vector{X: 1., Y: 2., Z: 3.},
		r3.Vector{X: 0., Y: 1., Z: 0.},
		math.Pi/2,
	)

	transformMsgs := []*commonpb.Transform{
		{
			ReferenceFrame: "frame1",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "pieceArm",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "frame2",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "pieceGripper",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "frame2a",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "frame2",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "frame2c",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "frame2",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "frame3",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "world",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
	}
	fs, err := framesystem.RobotFrameSystem(context.Background(), r, transformMsgs)
	test.That(t, err, test.ShouldBeNil)
	// 4 frames defined + 5 from transforms, 18 frames when including the offset,
	test.That(t, len(fs.FrameNames()), test.ShouldEqual, 18)

	// see if all frames are present and if their frames are correct
	test.That(t, fs.GetFrame("world"), test.ShouldNotBeNil)

	t.Log("pieceArm")
	test.That(t, fs.GetFrame("pieceArm"), test.ShouldNotBeNil)
	pose, err := fs.GetFrame("pieceArm").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{500, 0, 300})

	t.Log("pieceArm_offset")
	test.That(t, fs.GetFrame("pieceArm_offset"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("pieceArm_offset").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{500, 500, 1000})

	t.Log("pieceGripper")
	test.That(t, fs.GetFrame("pieceGripper"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("pieceGripper").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 200})

	t.Log("pieceGripper_offset")
	test.That(t, fs.GetFrame("pieceGripper_offset"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("pieceGripper_offset").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	t.Log("gps2")
	test.That(t, fs.GetFrame("gps2"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("gps2").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	t.Log("gps2_offset")
	test.That(t, fs.GetFrame("gps2_offset"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("gps2_offset").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	t.Log("cameraOver")
	test.That(t, fs.GetFrame("cameraOver"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("cameraOver").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	t.Log("cameraOver_offset")
	test.That(t, fs.GetFrame("cameraOver_offset"), test.ShouldNotBeNil)
	pose, err = fs.GetFrame("cameraOver_offset").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{2000, 500, 1300})

	t.Log("gps1")
	test.That(t, fs.GetFrame("gps1"), test.ShouldBeNil) // gps1 is not registered

	// There is a point at (1500, 500, 1300) in the world referenceframe. See if it transforms correctly in each referenceframe.
	worldPt := r3.Vector{1500, 500, 1300}
	armPt := r3.Vector{0, 0, 500}
	transformPoint, err := fs.TransformPoint(blankPos, worldPt, referenceframe.World, "pieceArm")
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, armPt)

	sensorPt := r3.Vector{0, 0, 500}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, referenceframe.World, "gps2")
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, sensorPt)

	gripperPt := r3.Vector{0, 0, 300}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, referenceframe.World, "pieceGripper")
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, gripperPt)

	cameraPt := r3.Vector{500, 0, 0}
	transformPoint, err = fs.TransformPoint(blankPos, worldPt, referenceframe.World, "cameraOver")
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, cameraPt)

	// go from camera point to gripper point
	transformPoint, err = fs.TransformPoint(blankPos, cameraPt, "cameraOver", "pieceGripper")
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, transformPoint, gripperPt)
}

// All of these config files should fail.
func TestWrongFrameSystems(t *testing.T) {
	// use impl/data/fake_wrongconfig*.json as config input
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), rdkutils.ResolveFile("robot/impl/data/fake_wrongconfig2.json"), logger) // no world node
	test.That(t, err, test.ShouldBeNil)
	_, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeError, framesystem.NewMissingParentError("pieceArm", "base"))
	cfg, err = config.Read(
		context.Background(),
		rdkutils.ResolveFile("robot/impl/data/fake_wrongconfig3.json"),
		logger,
	) // one of the nodes was given the name world
	test.That(t, err, test.ShouldBeNil)
	_, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeError, errors.Errorf("cannot give frame system part the name %s", referenceframe.World))

	cfg, err = config.Read(
		context.Background(),
		rdkutils.ResolveFile("robot/impl/data/fake_wrongconfig4.json"),
		logger,
	) // the parent field was left empty for a component
	test.That(t, err, test.ShouldBeNil)
	_, err = robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeError, errors.New("parent field in frame config for part \"cameraOver\" is empty"))

	testPose := spatialmath.NewPoseFromAxisAngle(
		r3.Vector{X: 1., Y: 2., Z: 3.},
		r3.Vector{X: 0., Y: 1., Z: 0.},
		math.Pi/2,
	)

	transformMsgs := []*commonpb.Transform{
		{
			ReferenceFrame: "frame1",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "pieceArm",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "frame2",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "noParent",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
	}
	cfg, err = config.Read(context.Background(), rdkutils.ResolveFile("robot/impl/data/fake.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer r.Close(context.Background())

	fs, err := framesystem.RobotFrameSystem(context.Background(), r, transformMsgs)
	test.That(t, err, test.ShouldBeError, framesystem.NewMissingParentError("frame2", "noParent"))
	test.That(t, fs, test.ShouldBeNil)

	transformMsgs = []*commonpb.Transform{
		{
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "pieceArm",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
	}
	fs, err = framesystem.RobotFrameSystem(context.Background(), r, transformMsgs)
	test.That(t, err, test.ShouldBeError, config.NewMissingReferenceFrameError(&commonpb.Transform{}))
	test.That(t, fs, test.ShouldBeNil)
}

func TestServiceWithRemote(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// make the remote robots
	remoteConfig, err := config.Read(context.Background(), rdkutils.ResolveFile("robot/impl/data/fake.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	ctx := context.Background()
	remoteRobot, err := robotimpl.New(ctx, remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, remoteRobot.Close(context.Background()), test.ShouldBeNil)
	}()
	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := weboptions.New()
	options.Network.BindAddress = fmt.Sprintf("localhost:%d", port)
	err = robotimpl.StartWeb(ctx, remoteRobot, options)
	test.That(t, err, test.ShouldBeNil)
	addr := fmt.Sprintf("localhost:%d", port)

	// make the local robot
	localConfig := &config.Config{
		Components: []config.Component{
			{
				Name:  "foo",
				Type:  base.SubtypeName,
				Model: "fake",
				Frame: &config.Frame{
					Parent: referenceframe.World,
				},
			},
			{
				Name:  "myParentIsRemote",
				Type:  gripper.SubtypeName,
				Model: "fake",
				Frame: &config.Frame{
					Parent: "bar.pieceArm",
				},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "bar",
				Address: addr,
				Prefix:  true,
				Frame: &config.Frame{
					Parent:      "foo",
					Translation: spatialmath.TranslationConfig{100, 200, 300},
					Orientation: &spatialmath.R4AA{math.Pi / 2., 0, 0, 1},
				},
			},
			{
				Name:    "squee",
				Prefix:  false,
				Address: addr,
				Frame: &config.Frame{
					Parent:      referenceframe.World,
					Translation: spatialmath.TranslationConfig{500, 600, 700},
					Orientation: &spatialmath.R4AA{math.Pi / 2., 1, 0, 0},
				},
			},
			{
				Name:    "dontAddMe", // no frame info, should be skipped
				Prefix:  true,
				Address: addr,
			},
		},
	}

	// make local robot
	testPose := spatialmath.NewPoseFromAxisAngle(
		r3.Vector{X: 1., Y: 2., Z: 3.},
		r3.Vector{X: 0., Y: 1., Z: 0.},
		math.Pi/2,
	)

	transformMsgs := []*commonpb.Transform{
		{
			ReferenceFrame: "frame1",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "pieceArm",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "frame2",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "pieceGripper",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "frame2a",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "frame2",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "frame2c",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "frame2",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "frame3",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "world",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
	}

	r2, err := robotimpl.New(context.Background(), localConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	fs, err := framesystem.RobotFrameSystem(context.Background(), r2, transformMsgs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 34)
	// run the frame system service
	fsServ, err := framesystem.FromRobot(r2)
	test.That(t, err, test.ShouldBeNil)
	allParts, err := fsServ.Config(context.Background(), transformMsgs)
	test.That(t, err, test.ShouldBeNil)
	t.Logf("frame system:\n%v", allParts)
}

func pointAlmostEqual(t *testing.T, from, to r3.Vector) {
	t.Helper()
	test.That(t, from.X, test.ShouldAlmostEqual, to.X)
	test.That(t, from.Y, test.ShouldAlmostEqual, to.Y)
	test.That(t, from.Z, test.ShouldAlmostEqual, to.Z)
}
