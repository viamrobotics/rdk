package robotimpl

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/testutils"

	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/arm"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	internalcloud "go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin"
	// TODO(RSDK-7884): change all referenced resources to mocks.
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

func TestRemoteRobotsGold(t *testing.T) {
	t.Parallel()
	// This tests that a main part is able to start up with an offline remote robot, connect to it and
	// depend on the remote robot's resources when it comes online. And react appropriately when the remote robot goes offline again.

	// If a new robot object/process comes online at the same address+port, the main robot should still be able
	// to use the new remote robot's resources.

	// To do so, the test initially sets up two remote robots, Remote 1 and 2, and then a third remote, Remote 3,
	// in the following scenario:
	// 1) Remote 1's server is started.
	// 2) The main robot is then set up with resources that depend on resources on both Remote 1 and 2. Since
	//    Remote 2 is not up, their resources are not available to the main robot.
	// 3) After initial configuration, Remote 2's server starts up and the main robot should then connect
	//	  and pick up the new available resources.
	// 4) Remote 2 goes down, and the main robot should remove any resources or resources that depend on
	//    resources from Remote 2.
	// 5) Remote 3 comes online at the same address as Remote 2, and the main robot should treat it the same as
	//    if Remote 2 came online again and re-add all the removed resources.
	logger := logging.NewTestLogger(t)
	remoteConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "remoteArm",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API: arm.API,
			},
		},
	}

	ctx := context.Background()

	// set up and start remote1's web service
	remote1 := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote1"))
	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := remote1.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// set up but do not start remote2's web service
	remote2 := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote2"))
	options, listener2, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
	_ = addr2

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API:       arm.API,
				DependsOn: []string{"fooremoteArm"},
			},
			{
				Name:  "arm2",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API:       arm.API,
				DependsOn: []string{"barremoteArm"},
			},
		},
		Services: []resource.Config{},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Prefix:  "foo",
				Address: addr1,
			},
			{
				Name:    "bar",
				Prefix:  "bar",
				Address: addr2,
			},
		},
	}
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("main"))

	// assert all of remote1's resources exist on main but none of remote2's
	resourceNames := r.ResourceNames()
	rdktestutils.VerifySameResourceNames(
		t,
		resourceNames,
		[]resource.Name{
			arm.Named("arm1"),
			arm.Named("foo:fooremoteArm"),
		},
	)

	// start remote2's web service
	err = remote2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	mainPartAndFooAndBarResources := []resource.Name{
		arm.Named("arm1"),
		arm.Named("arm2"),
		arm.Named("foo:fooremoteArm"),
		arm.Named("bar:barremoteArm"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		resourceNames := r.ResourceNames()
		rdktestutils.VerifySameResourceNames(
			tb,
			resourceNames,
			mainPartAndFooAndBarResources,
		)
	})
	test.That(t, remote2.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		verifyReachableResourceNames(tb, r,
			[]resource.Name{
				arm.Named("arm1"),
				arm.Named("foo:fooremoteArm"),
			},
		)
	})

	remote3 := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote3"))

	// Note: There's a slight chance this test can fail if someone else
	// claims the port we just released by closing the server.
	listener2, err = net.Listen("tcp", listener2.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	options.Network.Listener = listener2
	err = remote3.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		resourceNames := r.ResourceNames()
		rdktestutils.VerifySameResourceNames(tb, resourceNames, mainPartAndFooAndBarResources)
	})
}

func TestRemoteRobotsUpdate(t *testing.T) {
	t.Parallel()
	// The test tests that the robot is able to update when multiple remote robot
	// updates happen at the same time.
	logger := logging.NewTestLogger(t)
	remoteConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API: arm.API,
			},
		},
	}
	ctx := context.Background()
	remote := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote"))

	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := remote.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	localConfig := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Prefix:  "foo",
				Address: addr1,
			},
			{
				Name:    "bar",
				Prefix:  "bar",
				Address: addr1,
			},
			{
				Name:    "hello",
				Prefix:  "hello",
				Address: addr1,
			},
			{
				Name:    "world",
				Prefix:  "world",
				Address: addr1,
			},
		},
	}
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))

	expectedSet := []resource.Name{
		arm.Named("foo:fooarm1"),
		arm.Named("bar:bararm1"),
		arm.Named("hello:helloarm1"),
		arm.Named("world:worldarm1"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})
	test.That(t, remote.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		verifyReachableResourceNames(tb, r,
			[]resource.Name{},
		)
	})
}

func TestInferRemoteRobotDependencyConnectAtStartup(t *testing.T) {
	t.Parallel()
	// The test tests that the robot is able to infer remote dependencies
	// if remote name is not part of the specified dependency
	// and the remote is online at start up.
	logger := logging.NewTestLogger(t)

	fooCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "pieceArm",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API: arm.API,
			},
		},
	}
	ctx := context.Background()
	foo := setupLocalRobot(t, ctx, fooCfg, logger.Sublogger("foo"))

	options, listener1, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := foo.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API:       arm.API,
				DependsOn: []string{"pieceArm"},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
		},
	}
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))
	expectedSet := []resource.Name{
		arm.Named("arm1"),
		arm.Named("foo:pieceArm"),
	}

	rdktestutils.VerifySameResourceNames(t, r.ResourceNames(), expectedSet)
	test.That(t, foo.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		verifyReachableResourceNames(tb, r,
			[]resource.Name{},
		)
	})

	foo2 := setupLocalRobot(t, ctx, fooCfg, logger.Sublogger("foo2"))

	// Note: There's a slight chance this test can fail if someone else
	// claims the port we just released by closing the server.
	listener1, err = net.Listen("tcp", listener1.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	options.Network.Listener = listener1
	err = foo2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})
}

func TestInferRemoteRobotDependencyConnectAfterStartup(t *testing.T) {
	t.Parallel()
	// The test tests that the robot is able to infer remote dependencies
	// if remote name is not part of the specified dependency
	// and the remote is offline at start up.
	logger := logging.NewTestLogger(t)

	fooCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "pieceArm",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API: arm.API,
			},
		},
	}

	ctx := context.Background()

	foo := setupLocalRobot(t, ctx, fooCfg, logger.Sublogger("foo"))

	options, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API:       arm.API,
				DependsOn: []string{"pieceArm"},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
		},
	}
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))
	rdktestutils.VerifySameResourceNames(t, r.ResourceNames(),
		[]resource.Name{},
	)
	err := foo.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	expectedSet := []resource.Name{
		arm.Named("arm1"),
		arm.Named("foo:pieceArm"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})
	test.That(t, foo.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		verifyReachableResourceNames(tb, r,
			[]resource.Name{},
		)
	})
}

func TestInferRemoteRobotDependencyAmbiguous(t *testing.T) {
	// The test tests that the robot will not build a resource if the dependency
	// is ambiguous. In this case, "pieceArm" can refer to both "foo:pieceArm"
	// and "bar:pieceArm".
	logger := logging.NewTestLogger(t)

	remoteCfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "pieceArm",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API: arm.API,
			},
		},
	}

	ctx := context.Background()

	foo := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("foo"))
	bar := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("bar"))

	options1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	err := foo.StartWeb(ctx, options1)
	test.That(t, err, test.ShouldBeNil)

	options2, _, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
	err = bar.StartWeb(ctx, options2)
	test.That(t, err, test.ShouldBeNil)

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API:       arm.API,
				DependsOn: []string{"pieceArm"},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
			{
				Name:    "bar",
				Address: addr2,
			},
		},
	}
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))

	// We expect the robot to correctly detect the ambiguous dependency and not
	// build the resource. The remote pieceArms will also not be included because
	// their names collide.
	rdktestutils.VerifySameResourceNames(t, r.ResourceNames(), []resource.Name{})

	// now reconfig to remove the ambiguity
	reConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/kinematics/fake.json",
				},
				API:       arm.API,
				DependsOn: []string{"pieceArm"},
			},
		},
		Remotes: []config.Remote{
			{
				Name:    "foo",
				Address: addr1,
			},
			{
				Name:    "bar",
				Prefix:  "bar",
				Address: addr2,
			},
		},
	}
	r.Reconfigure(ctx, reConfig)

	finalSet := []resource.Name{
		arm.Named("foo:pieceArm"),
		arm.Named("bar:barpieceArm"),
		arm.Named("arm1"),
	}

	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), finalSet)
	})
}

func TestFullResourceNameCollision(t *testing.T) {
	// Asserts the following as features of RSDK-11267/RSDK-11268:
	// - Duplicately-named local resources are not reachable through an SDK
	// - Duplicately-named remote resources are not reachable through an SDK
	// - Local resources with the same full name as a remote resource are "preferred"
	// - Logs for full resource name collisions are correctly generated by the resource
	//   manager
	// - Reconfiguring a remote to have a prefix can "solve" a full resource name collision

	// Set up three injected arms and track their IsMoving counts separately.
	var arm1IsMovingCount, arm2IsMovingCount, arm3IsMovingCount atomic.Int32
	arm1 := &inject.Arm{
		IsMovingFunc: func(context.Context) (bool, error) {
			arm1IsMovingCount.Add(1)
			return false, nil
		},
	}
	arm2 := &inject.Arm{
		IsMovingFunc: func(context.Context) (bool, error) {
			arm2IsMovingCount.Add(1)
			return false, nil
		},
	}
	arm3 := &inject.Arm{
		IsMovingFunc: func(context.Context) (bool, error) {
			arm3IsMovingCount.Add(1)
			return false, nil
		},
	}
	resetAllIsMovingCounts := func() {
		arm1IsMovingCount.Store(0)
		arm2IsMovingCount.Store(0)
		arm3IsMovingCount.Store(0)
	}

	// Use a consistent model and constructor for all three arms, but vary the actual arm
	// created based on a config value. This will help us make assertions on _where_
	// IsMoving requests are routing to. This is likely redundant with some of the tests
	// above, but we want a full integration test here.
	model := resource.DefaultModelFamily.WithModel(goutils.RandomAlphaString(8))
	resource.RegisterComponent(
		arm.API,
		model,
		resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (arm.Arm, error) {
			switch conf.Attributes.String("armToRouteTo") {
			case "arm1":
				return arm1, nil
			case "arm2":
				return arm2, nil
			case "arm3":
				return arm3, nil
			default:
				return nil, errors.New("unknown armToRouteTo provided")
			}
		}},
	)
	defer resource.Deregister(arm.API, model)

	logger, logs := logging.NewObservedTestLogger(t)
	blankLogger := logging.NewBlankLogger("") // To be used where we don't care about logging.
	ctx := context.Background()

	// Setup three machines.
	r := setupLocalRobot(t, ctx, &config.Config{}, logger)
	options, _, raddr := robottestutils.CreateBaseOptionsAndListener(t)
	err := r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)
	r2 := setupLocalRobot(t, ctx, &config.Config{}, blankLogger)
	options, _, r2addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)
	r3 := setupLocalRobot(t, ctx, &config.Config{}, blankLogger)
	options, _, r3addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r3.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// Create clients to two of the machines (to be used throughout tests).
	rClient := robottestutils.NewRobotClient(t, blankLogger, raddr, time.Second)
	r2Client := robottestutils.NewRobotClient(t, blankLogger, r2addr, time.Second)

	{ // Single local "fooArm" instance.
		cfg := &config.Config{
			Components: []resource.Config{
				{
					Name:       "fooArm",
					Model:      model,
					API:        arm.API,
					Attributes: utils.AttributeMap{"armToRouteTo": "arm1"},
				},
			},
		}
		r.Reconfigure(ctx, cfg)

		// Verify no logs were generated complaining about collisions.
		test.That(t, logs.FilterMessageSnippet("collision").Len(), test.ShouldEqual, 0)

		// Verify (refreshed) resource names returned to rClient contain only one "fooArm" instance.
		expectedNames := []resource.Name{arm.Named("fooArm")}
		test.That(t, rClient.Refresh(ctx), test.ShouldBeNil)
		actualNames := rClient.ResourceNames()
		rdktestutils.VerifySameResourceNames(t, actualNames, expectedNames)

		// Call IsMoving() through "fooArm" from rClient, and assert that request routed to
		// arm1.
		fooArmResClient, err := rClient.ResourceByName(arm.Named("fooArm"))
		test.That(t, err, test.ShouldBeNil)
		fooArmClient, ok := fooArmResClient.(arm.Arm)
		test.That(t, ok, test.ShouldBeTrue)
		isMoving, err := fooArmClient.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeFalse)
		test.That(t, arm1IsMovingCount.Load(), test.ShouldEqual, 1)
	}

	logs.TakeAll() // Reset logs.
	resetAllIsMovingCounts()

	{ // Duplicate local "fooArm" instances.
		cfg := &config.Config{
			Components: []resource.Config{
				{
					Name:       "fooArm",
					Model:      model,
					API:        arm.API,
					Attributes: utils.AttributeMap{"armToRouteTo": "arm1"},
				},
				{
					Name:       "fooArm",
					Model:      model,
					API:        arm.API,
					Attributes: utils.AttributeMap{"armToRouteTo": "arm1"},
				},
			},
		}
		r.Reconfigure(ctx, cfg)

		// Verify that a log was generated complaining about a collision.
		test.That(t, logs.FilterMessageSnippet("collision").Len(), test.ShouldEqual, 1)

		// Verify (refreshed) resource names returned to rClient contain _no_ "fooArm" instances.
		expectedNames := []resource.Name{}
		test.That(t, rClient.Refresh(ctx), test.ShouldBeNil)
		actualNames := rClient.ResourceNames()
		rdktestutils.VerifySameResourceNames(t, actualNames, expectedNames)

		// Verify that "fooArm" can no longer be reached through ResourceByName from the rClient.
		_, err := rClient.ResourceByName(arm.Named("fooArm"))
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
	}

	logs.TakeAll() // Reset logs.
	resetAllIsMovingCounts()

	{ // One local "fooArm" instance and one remote "fooArm" instance.
		cfg2 := &config.Config{
			Components: []resource.Config{
				{
					Name:       "fooArm",
					Model:      model,
					API:        arm.API,
					Attributes: utils.AttributeMap{"armToRouteTo": "arm2"},
				},
			},
		}
		r2.Reconfigure(ctx, cfg2)
		cfg := &config.Config{
			Components: []resource.Config{
				{
					Name:       "fooArm",
					Model:      model,
					API:        arm.API,
					Attributes: utils.AttributeMap{"armToRouteTo": "arm1"},
				},
			},
			Remotes: []config.Remote{
				{
					Name:    "r2",
					Address: r2addr,
				},
			},
		}
		r.Reconfigure(ctx, cfg)

		// Verify that a log was generated complaining about a collision.
		test.That(t, logs.FilterMessageSnippet("collision").Len(), test.ShouldEqual, 1)

		// Verify (refreshed) resource names returned to rClient contain only one "fooArm"
		// instance (local).
		expectedNames := []resource.Name{arm.Named("fooArm")}
		test.That(t, rClient.Refresh(ctx), test.ShouldBeNil)
		actualNames := rClient.ResourceNames()
		rdktestutils.VerifySameResourceNames(t, actualNames, expectedNames)

		// Call IsMoving() through "fooArm" from rClient, and assert that request routed to
		// arm1.
		fooArmResClient, err := rClient.ResourceByName(arm.Named("fooArm"))
		test.That(t, err, test.ShouldBeNil)
		fooArmClient, ok := fooArmResClient.(arm.Arm)
		test.That(t, ok, test.ShouldBeTrue)
		isMoving, err := fooArmClient.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeFalse)
		test.That(t, arm1IsMovingCount.Load(), test.ShouldEqual, 1 /* increase */)
		test.That(t, arm2IsMovingCount.Load(), test.ShouldEqual, 0 /* no increase */)

		// Verify (refreshed) resource names returned to _r2Client_ also contains only one
		// "fooArm" instance (local to r2, but remote to main machine).
		expectedNames = []resource.Name{arm.Named("fooArm")}
		test.That(t, r2Client.Refresh(ctx), test.ShouldBeNil)
		actualNames = r2Client.ResourceNames()
		rdktestutils.VerifySameResourceNames(t, actualNames, expectedNames)

		// Call IsMoving() through "fooArm" from _r2Client_, and assert that request routed to
		// arm2.
		fooArmRes2Client, err := r2Client.ResourceByName(arm.Named("fooArm"))
		test.That(t, err, test.ShouldBeNil)
		fooArm2Client, ok := fooArmRes2Client.(arm.Arm)
		test.That(t, ok, test.ShouldBeTrue)
		isMoving, err = fooArm2Client.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeFalse)
		test.That(t, arm1IsMovingCount.Load(), test.ShouldEqual, 1 /* no increase */)
		test.That(t, arm2IsMovingCount.Load(), test.ShouldEqual, 1 /* increase */)

		// Assert that prefixing the remote produces no new collision errors, and both arm1
		// and arm2 are now reachable through rClient.
		cfg.Remotes[0].Prefix = "r2"
		r.Reconfigure(ctx, cfg)

		test.That(t, logs.FilterMessageSnippet("collision").Len(), test.ShouldEqual, 1 /* no increase */)

		expectedNames = []resource.Name{arm.Named("fooArm"), arm.Named("r2fooArm")}
		test.That(t, rClient.Refresh(ctx), test.ShouldBeNil)
		actualNames = rClient.ResourceNames()
		rdktestutils.VerifySameResourceNames(t, actualNames, expectedNames)

		// Assert that IsMoving calls route correctly.
		isMoving, err = fooArmClient.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeFalse)
		test.That(t, arm1IsMovingCount.Load(), test.ShouldEqual, 2 /* increase */)
		test.That(t, arm2IsMovingCount.Load(), test.ShouldEqual, 1 /* no increase */)

		r2fooArmResClient, err := rClient.ResourceByName(arm.Named("r2fooArm"))
		test.That(t, err, test.ShouldBeNil)
		r2fooArmClient, ok := r2fooArmResClient.(arm.Arm)
		test.That(t, ok, test.ShouldBeTrue)
		isMoving, err = r2fooArmClient.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeFalse)
		test.That(t, arm1IsMovingCount.Load(), test.ShouldEqual, 2 /* no increase */)
		test.That(t, arm2IsMovingCount.Load(), test.ShouldEqual, 2 /* increase */)
	}

	logs.TakeAll() // Reset logs.
	resetAllIsMovingCounts()

	{ // One local "fooArm" instance and two remote "fooArm" instances.
		cfg3 := &config.Config{
			Components: []resource.Config{
				{
					Name:       "fooArm",
					Model:      model,
					API:        arm.API,
					Attributes: utils.AttributeMap{"armToRouteTo": "arm3"},
				},
			},
		}
		r3.Reconfigure(ctx, cfg3)
		cfg2 := &config.Config{
			Components: []resource.Config{
				{
					Name:       "fooArm",
					Model:      model,
					API:        arm.API,
					Attributes: utils.AttributeMap{"armToRouteTo": "arm2"},
				},
			},
		}
		r2.Reconfigure(ctx, cfg2)
		cfg := &config.Config{
			Components: []resource.Config{
				{
					Name:       "fooArm",
					Model:      model,
					API:        arm.API,
					Attributes: utils.AttributeMap{"armToRouteTo": "arm1"},
				},
			},
			Remotes: []config.Remote{
				{
					Name:    "r2",
					Address: r2addr,
				},
				{
					Name:    "r3",
					Address: r3addr,
				},
			},
		}
		r.Reconfigure(ctx, cfg)

		// Verify that one new log was generated complaining about collision as we added r3.
		test.That(t, logs.FilterMessageSnippet("collision").Len(), test.ShouldEqual, 1)

		// Verify (refreshed) resource names returned to rClient contain only one "fooArm"
		// instance (local).
		expectedNames := []resource.Name{arm.Named("fooArm")}
		test.That(t, rClient.Refresh(ctx), test.ShouldBeNil)
		actualNames := rClient.ResourceNames()
		rdktestutils.VerifySameResourceNames(t, actualNames, expectedNames)

		// Call IsMoving() through "fooArm" from rClient, and assert that request routed to
		// arm1 (local).
		fooArmResClient, err := rClient.ResourceByName(arm.Named("fooArm"))
		test.That(t, err, test.ShouldBeNil)
		fooArmClient, ok := fooArmResClient.(arm.Arm)
		test.That(t, ok, test.ShouldBeTrue)
		isMoving, err := fooArmClient.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeFalse)
		test.That(t, arm1IsMovingCount.Load(), test.ShouldEqual, 1 /* increase */)
		test.That(t, arm2IsMovingCount.Load(), test.ShouldEqual, 0 /* no increase */)
		test.That(t, arm3IsMovingCount.Load(), test.ShouldEqual, 0 /* no increase */)

		// Assert that removing "fooArm" from the main machine leaves neither of the remote
		// "fooArm"s accessible through the main machine due to name collision.
		cfg.Components = nil
		r.Reconfigure(ctx, cfg)

		test.That(t, logs.FilterMessageSnippet("collision").Len(), test.ShouldEqual, 1 /* no increase */)

		expectedNames = []resource.Name{}
		test.That(t, rClient.Refresh(ctx), test.ShouldBeNil)
		actualNames = rClient.ResourceNames()
		rdktestutils.VerifySameResourceNames(t, actualNames, expectedNames)

		// Verify that "fooArm" can no longer be reached through ResourceByName from the rClient.
		_, err = rClient.ResourceByName(arm.Named("fooArm"))
		test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)

		// Assert that prefixing one of the remotes allows access to both arm2 and arm3
		// through the main machine.
		cfg.Remotes[0].Prefix = "r2"
		r.Reconfigure(ctx, cfg)

		expectedNames = []resource.Name{arm.Named("fooArm"), arm.Named("r2fooArm")}
		test.That(t, rClient.Refresh(ctx), test.ShouldBeNil)
		actualNames = rClient.ResourceNames()
		rdktestutils.VerifySameResourceNames(t, actualNames, expectedNames)

		// Assert that IsMoving calls route correctly ("fooArm" to r3, and "r2fooArm" to r2).
		fooArmResClient, err = rClient.ResourceByName(arm.Named("fooArm"))
		test.That(t, err, test.ShouldBeNil)
		fooArmClient, ok = fooArmResClient.(arm.Arm)
		test.That(t, ok, test.ShouldBeTrue)
		isMoving, err = fooArmClient.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeFalse)
		test.That(t, arm1IsMovingCount.Load(), test.ShouldEqual, 1 /* no increase */)
		test.That(t, arm2IsMovingCount.Load(), test.ShouldEqual, 0 /* no increase */)
		test.That(t, arm3IsMovingCount.Load(), test.ShouldEqual, 1 /* increase */)

		r2fooArmResClient, err := rClient.ResourceByName(arm.Named("r2fooArm"))
		test.That(t, err, test.ShouldBeNil)
		r2fooArmClient, ok := r2fooArmResClient.(arm.Arm)
		test.That(t, ok, test.ShouldBeTrue)
		isMoving, err = r2fooArmClient.IsMoving(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, isMoving, test.ShouldBeFalse)
		test.That(t, arm1IsMovingCount.Load(), test.ShouldEqual, 1 /* no increase */)
		test.That(t, arm2IsMovingCount.Load(), test.ShouldEqual, 1 /* increase */)
		test.That(t, arm3IsMovingCount.Load(), test.ShouldEqual, 1 /* no increase */)
	}
}

func TestRemoteCaptureMethodsName(t *testing.T) {
	// Primarily a regression test for RSDK-13349.
	//
	// Setup two machines:
	// - machineA with a sensor "foo"
	// - machineB with machineA as a remote and data capture on machineA's "foo" sensor
	//
	// Ensure that machineB's datamanager instance is able to access the "foo" sensor via
	// the capture methods of its associated resource config. Try two scenarios: one where
	// machineA has a prefix in machineB's config, and one where it does not.

	testRemoteCaptureMethodsName(t, "")
	testRemoteCaptureMethodsName(t, "machineA:")
}

func testRemoteCaptureMethodsName(t *testing.T, machineAPrefix string) {
	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	machineACfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "foo",
				API:   sensor.API,
				Model: resource.DefaultModelFamily.WithModel("fake"),
			},
		},
	}
	machineA := setupLocalRobot(t, ctx, machineACfg, logger)
	options, _, machineAAddr := robottestutils.CreateBaseOptionsAndListener(t)
	err := machineA.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	unprocessedMachineBCfg := &config.Config{
		Services: []resource.Config{
			{
				Name:                "datamanager",
				Model:               resource.DefaultServiceModel,
				API:                 datamanager.API,
				ConvertedAttributes: &builtin.Config{},
				DependsOn:           []string{internalcloud.InternalServiceName.String()},
			},
		},
		Remotes: []config.Remote{
			{
				Prefix:  machineAPrefix,
				Name:    "machineA",
				Address: machineAAddr,
				AssociatedResourceConfigs: []resource.AssociatedResourceConfig{
					{
						API: datamanager.API,
						Attributes: utils.AttributeMap{
							"capture_methods": []any{
								map[string]any{
									"name":   "rdk:component:sensor/foo",
									"method": "Readings",
								},
							},
						},
					},
				},
			},
		},
	}
	// "Process" the config before setting up a machine. processConfig has the side of
	// creating the appropriate converted attributes for the associated resource config
	// defined above.
	machineBCfg, err := config.ProcessLocalConfigForTesting(unprocessedMachineBCfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_ = setupLocalRobot(t, ctx, machineBCfg, logger)

	// Assert that there were no logs about datamanager failing to lookup the resource from
	// dependencies.
	test.That(t, logs.FilterMessageSnippet("datamanager failed to lookup resource from config").Len(),
		test.ShouldEqual, 0)
}
