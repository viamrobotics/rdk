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
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	_ "go.viam.com/rdk/services/datamanager/builtin"
	"go.viam.com/rdk/spatialmath"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	rutils "go.viam.com/rdk/utils"
)

func TestFrameSystemConfigWithRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)
	// make the remote robots
	remoteConfig, err := config.Read(context.Background(), rutils.ResolveFile("robot/impl/data/fake.json"), logger.Sublogger("remote"), nil)
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
					Parent: "barpieceArm",
				},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "bar",
				Address: addr,
				Prefix:  "bar",
				Frame: &referenceframe.LinkConfig{
					Parent:      "foo",
					Translation: r3.Vector{100, 200, 300},
					Orientation: o1Cfg,
				},
			},
			{
				Name:    "squee",
				Address: addr,
				Prefix:  "squee",
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
		referenceframe.NewLinkInFrame("barpieceArm", testPose, "frame1", nil),
		referenceframe.NewLinkInFrame("barpieceGripper", testPose, "frame2", nil),
		referenceframe.NewLinkInFrame("frame2", testPose, "frame2a", nil),
		referenceframe.NewLinkInFrame("frame2", testPose, "frame2c", nil),
		referenceframe.NewLinkInFrame(referenceframe.World, testPose, "frame3", nil),
	}
	r2 := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))

	test.That(t, err, test.ShouldBeNil)
	fsCfg, err := r2.FrameSystemConfig(context.Background())
	test.That(t, err, test.ShouldBeNil)
	fs, err := referenceframe.NewFrameSystem("", fsCfg.Parts, transforms)

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
	test.That(t, err.Error(), test.ShouldContainSubstring,
		"Cannot construct frame system. Some parts are not linked to the world frame")

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
				Prefix:  "bar",
				Address: addr,
				Frame: &referenceframe.LinkConfig{
					Parent:      "foo",
					Translation: r3.Vector{100, 200, 300},
					Orientation: o1Cfg,
				},
			},
			{
				Name:    "squee",
				Prefix:  "squee",
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

func TestModularFramesystemDependency(t *testing.T) {
	// Primarily a regression test for RSDK-9430/12124. Ensures that a modular resource can
	// depend on the framesystem service and use the service through that dependency,
	// whether through the constructor or the Reconfigure method.
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	testFSDependentModel := resource.NewModel("rdk", "test", "fsdep")
	testPath := rtestutils.BuildTempModule(t, "module/testmodule")

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "fsDep",
				Model: testFSDependentModel,
				API:   generic.API,
			},
			{
				Name:  "foo",
				API:   base.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
				Frame: &referenceframe.LinkConfig{
					Parent: referenceframe.World,
				},
			},
			{
				Name:  "myParentIsFoo",
				API:   gripper.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
				Frame: &referenceframe.LinkConfig{
					Parent: "foo",
				},
			},
		},
		Modules: []config.Module{
			{
				Name:    "mod",
				ExePath: testPath,
			},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	fsDep, err := r.ResourceByName(generic.Named("fsDep"))
	test.That(t, err, test.ShouldBeNil)

	resp, err := fsDep.DoCommand(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["fsCfg"], test.ShouldNotBeNil)
	test.That(t, resp["fsCfg"], test.ShouldContainSubstring, "foo")
	test.That(t, resp["fsCfg"], test.ShouldContainSubstring, "myParentIsFoo")

	// Do a meaningless reconfigure (random attributes) to force a ReconfigureResource call
	// to `fsDep` that will try to grab the framesystem from the passed in deps.
	cfg.Components[0].Attributes = rutils.AttributeMap{"not": "used"}
	r.Reconfigure(ctx, cfg)

	// Assert that `fsDep` is still reachable and usable.
	fsDep, err = r.ResourceByName(generic.Named("fsDep"))
	test.That(t, err, test.ShouldBeNil)

	resp, err = fsDep.DoCommand(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["fsCfg"], test.ShouldNotBeNil)
	test.That(t, resp["fsCfg"], test.ShouldContainSubstring, "foo")
	test.That(t, resp["fsCfg"], test.ShouldContainSubstring, "myParentIsFoo")
}
