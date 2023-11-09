package framesystem_test

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/gripper"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/robottestutils"
	rdkutils "go.viam.com/rdk/utils"
)

func TestEmptyConfigFrameService(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	r, err := robotimpl.New(ctx, &config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)
	fsCfg, err := r.FrameSystemConfig(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsCfg.Parts, test.ShouldHaveLength, 0)
	fs, err := referenceframe.NewFrameSystem("test", fsCfg.Parts, fsCfg.AdditionalTransforms)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 0)
}

func TestNewFrameSystemFromConfig(t *testing.T) {
	o1 := &spatialmath.R4AA{Theta: math.Pi / 2, RZ: 1}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
	test.That(t, err, test.ShouldBeNil)

	l1 := &referenceframe.LinkConfig{
		ID:          "frame1",
		Parent:      referenceframe.World,
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
		Orientation: o1Cfg,
		Geometry:    &spatialmath.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
	}
	lif1, err := l1.ParseConfig()
	test.That(t, err, test.ShouldBeNil)

	l2 := &referenceframe.LinkConfig{
		ID:          "frame2",
		Parent:      "frame1",
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
	}
	lif2, err := l2.ParseConfig()
	test.That(t, err, test.ShouldBeNil)

	parts := []*referenceframe.FrameSystemPart{
		{
			FrameConfig: lif1,
		},
		{
			FrameConfig: lif2,
		},
	}
	frameSys, err := referenceframe.NewFrameSystem("test", parts, []*referenceframe.LinkInFrame{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frameSys, test.ShouldNotBeNil)
	frame1 := frameSys.Frame("frame1")
	frame1Origin := frameSys.Frame("frame1_origin")
	frame2 := frameSys.Frame("frame2")
	frame2Origin := frameSys.Frame("frame2_origin")

	resFrame, err := frameSys.Parent(frame2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame2Origin)
	resFrame, err = frameSys.Parent(frame2Origin)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame1)
	resFrame, err = frameSys.Parent(frame1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frame1Origin)
	resFrame, err = frameSys.Parent(frame1Origin)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resFrame, test.ShouldResemble, frameSys.World())
}

func TestNewFrameSystemFromConfigWithTransforms(t *testing.T) {
	// use robot/impl/data/fake.json as config input
	ctx := context.Background()
	emptyIn := []referenceframe.Input{}
	zeroIn := []referenceframe.Input{{Value: 0.0}}
	blankPos := make(map[string][]referenceframe.Input)
	blankPos["pieceArm"] = zeroIn
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), rdkutils.ResolveFile("robot/impl/data/fake.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer r.Close(context.Background())
	fsCfg, err := r.FrameSystemConfig(ctx)
	test.That(t, err, test.ShouldBeNil)

	// use fake registrations to have a FrameSystem return
	testPose := spatialmath.NewPose(
		r3.Vector{X: 1., Y: 2., Z: 3.},
		&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
	)

	fsCfg.AdditionalTransforms = []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame("pieceArm", testPose, "frame1", nil),
		referenceframe.NewLinkInFrame("pieceGripper", testPose, "frame2", nil),
		referenceframe.NewLinkInFrame("frame2", testPose, "frame2a", nil),
		referenceframe.NewLinkInFrame("frame2", testPose, "frame2c", nil),
		referenceframe.NewLinkInFrame(referenceframe.World, testPose, "frame3", nil),
	}

	fs, err := referenceframe.NewFrameSystem("test", fsCfg.Parts, fsCfg.AdditionalTransforms)
	test.That(t, err, test.ShouldBeNil)
	// 4 frames defined + 5 from transforms, 18 frames when including the offset,
	test.That(t, len(fs.FrameNames()), test.ShouldEqual, 18)

	// see if all frames are present and if their frames are correct
	test.That(t, fs.Frame("world"), test.ShouldNotBeNil)

	t.Log("pieceArm")
	test.That(t, fs.Frame("pieceArm"), test.ShouldNotBeNil)
	pose, err := fs.Frame("pieceArm").Transform(zeroIn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Point().ApproxEqual(r3.Vector{500, 0, 300}), test.ShouldBeTrue)

	t.Log("pieceArm_origin")
	test.That(t, fs.Frame("pieceArm_origin"), test.ShouldNotBeNil)
	pose, err = fs.Frame("pieceArm_origin").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Point().ApproxEqual(r3.Vector{500, 500, 1000}), test.ShouldBeTrue)

	t.Log("pieceGripper")
	test.That(t, fs.Frame("pieceGripper"), test.ShouldNotBeNil)
	pose, err = fs.Frame("pieceGripper").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Point().ApproxEqual(r3.Vector{}), test.ShouldBeTrue)

	t.Log("pieceGripper_origin")
	test.That(t, fs.Frame("pieceGripper_origin"), test.ShouldNotBeNil)
	pose, err = fs.Frame("pieceGripper_origin").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Point().ApproxEqual(r3.Vector{}), test.ShouldBeTrue)

	t.Log("movement_sensor2")
	test.That(t, fs.Frame("movement_sensor2"), test.ShouldNotBeNil)
	pose, err = fs.Frame("movement_sensor2").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Point().ApproxEqual(r3.Vector{}), test.ShouldBeTrue)

	t.Log("movement_sensor2_origin")
	test.That(t, fs.Frame("movement_sensor2_origin"), test.ShouldNotBeNil)
	pose, err = fs.Frame("movement_sensor2_origin").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Point().ApproxEqual(r3.Vector{}), test.ShouldBeTrue)

	t.Log("cameraOver")
	test.That(t, fs.Frame("cameraOver"), test.ShouldNotBeNil)
	pose, err = fs.Frame("cameraOver").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Point().ApproxEqual(r3.Vector{}), test.ShouldBeTrue)

	t.Log("cameraOver_origin")
	test.That(t, fs.Frame("cameraOver_origin"), test.ShouldNotBeNil)
	pose, err = fs.Frame("cameraOver_origin").Transform(emptyIn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Point().ApproxEqual(r3.Vector{2000, 500, 1300}), test.ShouldBeTrue)

	t.Log("movement_sensor1")
	test.That(t, fs.Frame("movement_sensor1"), test.ShouldBeNil) // movement_sensor1 is not registered

	// There is a point at (1500, 500, 1300) in the world referenceframe. See if it transforms correctly in each referenceframe.
	worldPose := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{1500, 500, 1300}))
	armPt := r3.Vector{500, 0, 0}
	tf, err := fs.Transform(blankPos, worldPose, "pieceArm")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ := tf.(*referenceframe.PoseInFrame)
	test.That(t, transformPose.Pose().Point().ApproxEqual(armPt), test.ShouldBeTrue)

	sensorPt := r3.Vector{500, 0, 0}
	tf, err = fs.Transform(blankPos, worldPose, "movement_sensor2")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ = tf.(*referenceframe.PoseInFrame)
	test.That(t, transformPose.Pose().Point().ApproxEqual(sensorPt), test.ShouldBeTrue)

	gripperPt := r3.Vector{500, 0, 0}
	tf, err = fs.Transform(blankPos, worldPose, "pieceGripper")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ = tf.(*referenceframe.PoseInFrame)
	test.That(t, transformPose.Pose().Point().ApproxEqual(gripperPt), test.ShouldBeTrue)

	cameraPt := r3.Vector{500, 0, 0}
	tf, err = fs.Transform(blankPos, worldPose, "cameraOver")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ = tf.(*referenceframe.PoseInFrame)
	test.That(t, spatialmath.R3VectorAlmostEqual(transformPose.Pose().Point(), cameraPt, 1e-8), test.ShouldBeTrue)

	// go from camera point to gripper point
	cameraPose := referenceframe.NewPoseInFrame("cameraOver", spatialmath.NewPoseFromPoint(cameraPt))
	tf, err = fs.Transform(blankPos, cameraPose, "pieceGripper")
	test.That(t, err, test.ShouldBeNil)
	transformPose, _ = tf.(*referenceframe.PoseInFrame)
	test.That(t, spatialmath.R3VectorAlmostEqual(transformPose.Pose().Point(), gripperPt, 1e-8), test.ShouldBeTrue)
}

func TestNewFrameSystemFromBadConfig(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	testCases := []struct {
		name string
		num  string
		err  error
	}{
		{"no world node", "2", referenceframe.ErrNoWorldConnection},
		{"frame named world", "3", errors.Errorf("cannot give frame system part the name %s", referenceframe.World)},
		{"parent field empty", "4", errors.New("parent field in frame config for part \"cameraOver\" is empty")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.Read(ctx, rdkutils.ResolveFile("robot/impl/data/fake_wrongconfig"+tc.num+".json"), logger)
			test.That(t, err, test.ShouldBeNil)
			r, err := robotimpl.New(ctx, cfg, logger)
			test.That(t, err, test.ShouldBeNil)
			defer r.Close(ctx)
			fsCfg, err := r.FrameSystemConfig(ctx)
			if err != nil {
				test.That(t, err, test.ShouldBeError, tc.err)
				return
			}
			_, err = referenceframe.NewFrameSystem(tc.num, fsCfg.Parts, fsCfg.AdditionalTransforms)
			test.That(t, err, test.ShouldBeError, tc.err)
		})
	}

	cfg, err := config.Read(ctx, rdkutils.ResolveFile("robot/impl/data/fake.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer r.Close(ctx)

	testPose := spatialmath.NewPose(r3.Vector{X: 1., Y: 2., Z: 3.}, &spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.})

	t.Run("frame missing parent", func(t *testing.T) {
		transforms := []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame("pieceArm", testPose, "frame1", nil),
			referenceframe.NewLinkInFrame("noParent", testPose, "frame2", nil),
		}
		fsCfg, err := r.FrameSystemConfig(ctx)
		test.That(t, err, test.ShouldBeNil)
		fsCfg.AdditionalTransforms = transforms
		fs, err := referenceframe.NewFrameSystem("", fsCfg.Parts, fsCfg.AdditionalTransforms)
		test.That(t, err, test.ShouldBeError, referenceframe.NewParentFrameMissingError("frame2", "noParent"))
		test.That(t, fs, test.ShouldBeNil)
	})

	t.Run("empty string frame name", func(t *testing.T) {
		transforms := []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame("pieceArm", testPose, "", nil),
		}
		fsCfg, err := r.FrameSystemConfig(ctx)
		test.That(t, err, test.ShouldBeNil)
		fsCfg.AdditionalTransforms = transforms
		fs, err := referenceframe.NewFrameSystem("", fsCfg.Parts, fsCfg.AdditionalTransforms)
		test.That(t, err, test.ShouldBeError, referenceframe.ErrEmptyStringFrameName)
		test.That(t, fs, test.ShouldBeNil)
	})
}

func TestServiceWithRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)
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
		Components: []resource.Config{
			{
				Name:  "foo",
				API:   base.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
				Frame: &referenceframe.LinkConfig{
					Parent: referenceframe.World,
				},
			},
			{
				Name:  "myParentIsRemote",
				API:   gripper.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
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
	testPose := spatialmath.NewPose(
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
	fsCfg, err := r2.FrameSystemConfig(context.Background())
	test.That(t, err, test.ShouldBeNil)
	fsCfg.AdditionalTransforms = transforms
	fs, err := referenceframe.NewFrameSystem("test", fsCfg.Parts, fsCfg.AdditionalTransforms)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 34)
	// run the frame system service
	allParts, err := r2.FrameSystemConfig(context.Background())
	test.That(t, err, test.ShouldBeNil)
	t.Logf("frame system:\n%v", allParts)
	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
}
