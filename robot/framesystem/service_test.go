package framesystem_test

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/gripper"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
	rdkutils "go.viam.com/rdk/utils"
)

func TestFrameSystemFromConfig(t *testing.T) {
	// use robot/impl/data/fake.json as config input
	emptyIn := []referenceframe.Input{}
	zeroIn := []referenceframe.Input{{Value: 0.0}}
	blankPos := make(map[string][]referenceframe.Input)
	blankPos["pieceArm"] = zeroIn
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), rdkutils.ResolveFile("robot/impl/data/fake.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer r.Close(context.Background())

	// use fake registrations to have a FrameSystem return
	testPose := spatialmath.NewPoseFromOrientation(
		r3.Vector{X: 1., Y: 2., Z: 3.},
		&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
	)

	transforms := []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame("pieceArm", testPose, "frame1", nil),
		referenceframe.NewLinkInFrame("pieceGripper", testPose, "frame2", nil),
		referenceframe.NewLinkInFrame("frame2", testPose, "frame2a", nil),
		referenceframe.NewLinkInFrame("frame2", testPose, "frame2c", nil),
		referenceframe.NewLinkInFrame(referenceframe.World, testPose, "frame3", nil),
	}

	fs, err := framesystem.RobotFrameSystem(context.Background(), r, transforms)
	test.That(t, err, test.ShouldBeNil)
	// 4 frames defined + 5 from transforms, 18 frames when including the offset,
	test.That(t, len(fs.FrameNames()), test.ShouldEqual, 18)

	// see if all frames are present and if their frames are correct
	test.That(t, fs.Frame("world"), test.ShouldNotBeNil)

	t.Log("pieceArm")
	test.That(t, fs.Frame("pieceArm"), test.ShouldNotBeNil)
	pose, err := fs.Frame("pieceArm").Transform(zeroIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{500, 0, 300})

	t.Log("pieceArm_origin")
	test.That(t, fs.Frame("pieceArm_origin"), test.ShouldNotBeNil)
	pose, err = fs.Frame("pieceArm_origin").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{500, 500, 1000})

	t.Log("pieceGripper")
	test.That(t, fs.Frame("pieceGripper"), test.ShouldNotBeNil)
	pose, err = fs.Frame("pieceGripper").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	t.Log("pieceGripper_origin")
	test.That(t, fs.Frame("pieceGripper_origin"), test.ShouldNotBeNil)
	pose, err = fs.Frame("pieceGripper_origin").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	t.Log("movement_sensor2")
	test.That(t, fs.Frame("movement_sensor2"), test.ShouldNotBeNil)
	pose, err = fs.Frame("movement_sensor2").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	t.Log("movement_sensor2_origin")
	test.That(t, fs.Frame("movement_sensor2_origin"), test.ShouldNotBeNil)
	pose, err = fs.Frame("movement_sensor2_origin").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	t.Log("cameraOver")
	test.That(t, fs.Frame("cameraOver"), test.ShouldNotBeNil)
	pose, err = fs.Frame("cameraOver").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{0, 0, 0})

	t.Log("cameraOver_origin")
	test.That(t, fs.Frame("cameraOver_origin"), test.ShouldNotBeNil)
	pose, err = fs.Frame("cameraOver_origin").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	pointAlmostEqual(t, pose.Point(), r3.Vector{2000, 500, 1300})

	t.Log("movement_sensor1")
	test.That(t, fs.Frame("movement_sensor1"), test.ShouldBeNil) // movement_sensor1 is not registered

	// There is a point at (1500, 500, 1300) in the world referenceframe. See if it transforms correctly in each referenceframe.
	worldPose := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{1500, 500, 1300}))
	armPt := r3.Vector{500, 0, 0}
	tf, err := fs.Transform(blankPos, worldPose, "pieceArm")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ := tf.(*referenceframe.PoseInFrame)
	pointAlmostEqual(t, transformPose.Pose().Point(), armPt)

	sensorPt := r3.Vector{500, 0, 0}
	tf, err = fs.Transform(blankPos, worldPose, "movement_sensor2")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ = tf.(*referenceframe.PoseInFrame)
	pointAlmostEqual(t, transformPose.Pose().Point(), sensorPt)

	gripperPt := r3.Vector{500, 0, 0}
	tf, err = fs.Transform(blankPos, worldPose, "pieceGripper")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ = tf.(*referenceframe.PoseInFrame)
	pointAlmostEqual(t, transformPose.Pose().Point(), gripperPt)

	cameraPt := r3.Vector{500, 0, 0}
	tf, err = fs.Transform(blankPos, worldPose, "cameraOver")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ = tf.(*referenceframe.PoseInFrame)
	pointAlmostEqual(t, transformPose.Pose().Point(), cameraPt)

	// go from camera point to gripper point
	cameraPose := referenceframe.NewPoseInFrame("cameraOver", spatialmath.NewPoseFromPoint(cameraPt))
	tf, err = fs.Transform(blankPos, cameraPose, "pieceGripper")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ = tf.(*referenceframe.PoseInFrame)
	pointAlmostEqual(t, transformPose.Pose().Point(), gripperPt)
}

// All of these config files should fail.
func TestWrongFrameSystems(t *testing.T) {
	// use impl/data/fake_wrongconfig*.json as config input
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), rdkutils.ResolveFile("robot/impl/data/fake_wrongconfig2.json"), logger) // no world node
	test.That(t, err, test.ShouldBeNil)

	injectRobot := &inject.Robot{}
	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return cfg, nil
	}

	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return struct{}{}, nil
	}

	injectRobot.RemoteNamesFunc = func() []string {
		return []string{}
	}

	var resources map[resource.Name]interface{}
	ctx := context.Background()
	service := framesystem.New(ctx, injectRobot, logger)
	serviceUpdateable, ok := service.(resource.Updateable)
	test.That(t, ok, test.ShouldBeTrue)
	err = serviceUpdateable.Update(ctx, resources)
	test.That(t, err, test.ShouldBeError, framesystemparts.NewMissingParentError("pieceArm", "base"))
	cfg, err = config.Read(
		context.Background(),
		rdkutils.ResolveFile("robot/impl/data/fake_wrongconfig3.json"),
		logger,
	) // one of the nodes was given the name world
	test.That(t, err, test.ShouldBeNil)

	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return cfg, nil
	}
	err = serviceUpdateable.Update(ctx, resources)
	test.That(t, err, test.ShouldBeError, errors.Errorf("cannot give frame system part the name %s", referenceframe.World))

	cfg, err = config.Read(
		context.Background(),
		rdkutils.ResolveFile("robot/impl/data/fake_wrongconfig4.json"),
		logger,
	) // the parent field was left empty for a component
	test.That(t, err, test.ShouldBeNil)

	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) {
		return cfg, nil
	}
	err = serviceUpdateable.Update(ctx, resources)
	test.That(t, err, test.ShouldBeError, errors.New("parent field in frame config for part \"cameraOver\" is empty"))

	testPose := spatialmath.NewPoseFromOrientation(
		r3.Vector{X: 1., Y: 2., Z: 3.},
		&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
	)

	cfg, err = config.Read(context.Background(), rdkutils.ResolveFile("robot/impl/data/fake.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer r.Close(context.Background())

	transforms := []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame("pieceArm", testPose, "frame1", nil),
		referenceframe.NewLinkInFrame("noParent", testPose, "frame2", nil),
	}
	fs, err := framesystem.RobotFrameSystem(context.Background(), r, transforms)
	test.That(t, err, test.ShouldBeError, framesystemparts.NewMissingParentError("frame2", "noParent"))
	test.That(t, fs, test.ShouldBeNil)

	transforms = []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame("pieceArm", testPose, "", nil),
	}
	fs, err = framesystem.RobotFrameSystem(context.Background(), r, transforms)
	test.That(t, err, test.ShouldBeError, referenceframe.ErrEmptyStringFrameName)
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

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = remoteRobot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	o1 := &spatialmath.R4AA{math.Pi / 2., 0, 0, 1}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

	o2 := &spatialmath.R4AA{math.Pi / 2., 1, 0, 0}
	o2Cfg, err := spatialmath.NewOrientationConfig(o2)
	test.That(t, err, test.ShouldBeNil)

	// make the local robot
	localConfig := &config.Config{
		Components: []config.Component{
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "foo",
				Type:      base.SubtypeName,
				Model:     "fake",
				Frame: &referenceframe.LinkConfig{
					Parent: referenceframe.World,
				},
			},
			{
				Namespace: resource.ResourceNamespaceRDK,
				Name:      "myParentIsRemote",
				Type:      gripper.SubtypeName,
				Model:     "fake",
				Frame: &referenceframe.LinkConfig{
					Parent: "bar:pieceArm",
				},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "bar",
				Address: addr,
				Frame: &referenceframe.LinkConfig{
					Parent:      "foo",
					Translation: r3.Vector{100, 200, 300},
					Orientation: o1Cfg,
				},
			},
			{
				Name:    "squee",
				Address: addr,
				Frame: &referenceframe.LinkConfig{
					Parent:      referenceframe.World,
					Translation: r3.Vector{500, 600, 700},
					Orientation: o2Cfg,
				},
			},
			{
				Name:    "dontAddMe", // no frame info, should be skipped
				Address: addr,
			},
		},
	}

	// make local robot
	testPose := spatialmath.NewPoseFromOrientation(
		r3.Vector{X: 1., Y: 2., Z: 3.},
		&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
	)

	transforms := []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame("bar:pieceArm", testPose, "frame1", nil),
		referenceframe.NewLinkInFrame("bar:pieceGripper", testPose, "frame2", nil),
		referenceframe.NewLinkInFrame("frame2", testPose, "frame2a", nil),
		referenceframe.NewLinkInFrame("frame2", testPose, "frame2c", nil),
		referenceframe.NewLinkInFrame(referenceframe.World, testPose, "frame3", nil),
	}

	r2, err := robotimpl.New(context.Background(), localConfig, logger)
	test.That(t, err, test.ShouldBeNil)
	fs, err := framesystem.RobotFrameSystem(context.Background(), r2, transforms)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 34)
	// run the frame system service
	allParts, err := r2.FrameSystemConfig(context.Background(), transforms)
	test.That(t, err, test.ShouldBeNil)
	t.Logf("frame system:\n%v", allParts)
}

func pointAlmostEqual(t *testing.T, from, to r3.Vector) {
	t.Helper()
	test.That(t, from.X, test.ShouldAlmostEqual, to.X)
	test.That(t, from.Y, test.ShouldAlmostEqual, to.Y)
	test.That(t, from.Z, test.ShouldAlmostEqual, to.Z)
}
