package framesystem_test

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/gripper"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/robottestutils"
	rdkutils "go.viam.com/rdk/utils"
)

func TestEmptyConfigFrameService(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	r, err := robotimpl.New(ctx, &config.Config{}, logger)
	test.That(t, err, test.ShouldBeNil)
	fsCfg, err := r.FrameSystemConfig(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsCfg.Parts, test.ShouldHaveLength, 0)
	fs, err := framesystem.NewFrameSystemFromConfig("test", fsCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 0)
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
	fs, err := framesystem.NewFrameSystemFromConfig("test", fsCfg)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 34)
	// run the frame system service
	allParts, err := r2.FrameSystemConfig(context.Background())
	test.That(t, err, test.ShouldBeNil)
	t.Logf("frame system:\n%v", allParts)
	test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
}
