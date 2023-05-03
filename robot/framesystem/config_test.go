package framesystem_test

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot/framesystem"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

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
	frameSys, err := framesystem.NewFrameSystemFromConfig("test", &framesystem.Config{Parts: parts})
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
	logger := golog.NewTestLogger(t)
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

	fs, err := framesystem.NewFrameSystemFromConfig("test", fsCfg)
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
	logger := golog.NewTestLogger(t)

	testCases := []struct {
		name string
		num  string
		err  error
	}{
		{"no world node", "2", framesystem.NoWorldConnectionError},
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
			test.That(t, err, test.ShouldBeNil)
			_, err = framesystem.NewFrameSystemFromConfig(tc.num, fsCfg)
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
		fs, err := framesystem.NewFrameSystemFromConfig("test", fsCfg)
		test.That(t, err, test.ShouldBeError, framesystem.MissingParentError("frame2", "noParent"))
		test.That(t, fs, test.ShouldBeNil)
	})

	t.Run("empty string frame name", func(t *testing.T) {
		transforms := []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame("pieceArm", testPose, "", nil),
		}
		fsCfg, err := r.FrameSystemConfig(ctx)
		test.That(t, err, test.ShouldBeNil)
		fsCfg.AdditionalTransforms = transforms
		fs, err := framesystem.NewFrameSystemFromConfig("test", fsCfg)
		test.That(t, err, test.ShouldBeError, referenceframe.ErrEmptyStringFrameName)
		test.That(t, fs, test.ShouldBeNil)
	})
}
