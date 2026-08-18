package robotimpl

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
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
					ModelFilePath: "../../components/arm/kinematics/fake.json",
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

	// set up but do not start remote2's web service. Hold remote2's port so it
	// survives remote2's Close below and remote3 can reuse the exact same socket,
	// with no window for another process to claim the port in between.
	remote2 := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote2"))
	options, lis2, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
	hold2 := rdktestutils.HoldPort(t, lis2)
	options.Network.Listener = hold2

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/kinematics/fake.json",
				},
				API:       arm.API,
				DependsOn: []string{"fooremoteArm"},
			},
			{
				Name:  "arm2",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/kinematics/fake.json",
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
	// remote2's Close stops its server but, via holdPort, keeps the port bound.
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

	// Re-arm the held listener and bring remote3 up on the very same socket remote2
	// used. The port was never released, so there was no chance for it to be claimed.
	hold2.Rearm(t)
	remote3 := setupLocalRobot(t, ctx, remoteConfig, logger.Sublogger("remote3"))
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
					ModelFilePath: "../../components/arm/kinematics/fake.json",
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
					ModelFilePath: "../../components/arm/kinematics/fake.json",
				},
				API: arm.API,
			},
		},
	}
	ctx := context.Background()
	foo := setupLocalRobot(t, ctx, fooCfg, logger.Sublogger("foo"))

	// Hold foo's port so it survives foo's Close below and foo2 can reuse the exact
	// same socket, with no window for another process to claim the port in between.
	options, lis, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	hold := rdktestutils.HoldPort(t, lis)
	options.Network.Listener = hold
	err := foo.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/kinematics/fake.json",
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
	// foo's Close stops its server but, via holdPort, keeps the port bound.
	test.That(t, foo.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		verifyReachableResourceNames(tb, r,
			[]resource.Name{},
		)
	})

	// Re-arm the held listener and bring foo2 up on the very same socket foo used.
	hold.Rearm(t)
	foo2 := setupLocalRobot(t, ctx, fooCfg, logger.Sublogger("foo2"))
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
					ModelFilePath: "../../components/arm/kinematics/fake.json",
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
					ModelFilePath: "../../components/arm/kinematics/fake.json",
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
					ModelFilePath: "../../components/arm/kinematics/fake.json",
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
					ModelFilePath: "../../components/arm/kinematics/fake.json",
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
					ModelFilePath: "../../components/arm/kinematics/fake.json",
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
