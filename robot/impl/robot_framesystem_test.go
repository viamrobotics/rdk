package robotimpl

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	_ "go.viam.com/rdk/services/datamanager/builtin"
	"go.viam.com/rdk/services/motion"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

func TestFrameSystemConfigWithRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)
	// make the remote robots
	remoteConfig, err := config.Read(context.Background(), rutils.ResolveFile("robot/impl/data/fake.json"), logger.Sublogger("remote"))
	test.That(t, err, test.ShouldBeNil)
	ctx := context.Background()
	remoteRobot := setupLocalRobot(t, ctx, remoteConfig, logger)
	test.That(t, err, test.ShouldBeNil)

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
	r2 := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))

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

	test.That(t, remoteRobot.Close(context.Background()), test.ShouldBeNil)

	rr, ok := r2.(*localRobot)
	test.That(t, ok, test.ShouldBeTrue)

	rr.triggerConfig <- struct{}{}

	finalSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		base.Named("foo"),
		gripper.Named("myParentIsRemote"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		verifyReachableResourceNames(tb, r2, finalSet)
	})

	fsCfg, err = r2.FrameSystemConfig(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// expected error as remote parent frame is missing
	_, err = referenceframe.NewFrameSystem("test", fsCfg.Parts, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "references non-existent parent")

	// reconfigure to no longer have remote parent frame
	localConfig = &config.Config{
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
					Parent: referenceframe.World,
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
	r2.Reconfigure(context.Background(), localConfig)

	fsCfg, err = r2.FrameSystemConfig(context.Background())
	test.That(t, err, test.ShouldBeNil)
	fs, err = referenceframe.NewFrameSystem("test", fsCfg.Parts, nil)
	test.That(t, err, test.ShouldBeNil)
	t.Logf("frame system:\n%v", fsCfg)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 4)
}

func TestServiceWithUnavailableRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)
	o1 := &spatialmath.R4AA{math.Pi / 2., 0, 0, 1}
	o1Cfg, err := spatialmath.NewOrientationConfig(o1)
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
		},
		Remotes: []config.Remote{
			{
				Name:    "bar",
				Address: "addr",
				Frame: &referenceframe.LinkConfig{
					Parent:      "foo",
					Translation: r3.Vector{100, 200, 300},
					Orientation: o1Cfg,
				},
			},
		},
	}

	r := setupLocalRobot(t, context.Background(), localConfig, logger, withDisableCompleteConfigWorker())

	// make sure calling into remotes don't error
	fsCfg, err := r.FrameSystemConfig(context.Background())
	test.That(t, err, test.ShouldBeNil)

	fs, err := referenceframe.NewFrameSystem("test", fsCfg.Parts, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.FrameNames(), test.ShouldHaveLength, 2)
}
