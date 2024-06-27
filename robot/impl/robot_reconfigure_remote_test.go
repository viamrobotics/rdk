package robotimpl

import (
	"context"
	"net"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/arm"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	// TODO(RSDK-7884): change everything that depends on this import to a mock.
	"go.viam.com/rdk/services/motion"
	// TODO(RSDK-7884): change all referenced resources to mocks.
	"go.viam.com/rdk/services/sensors"
	rdktestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
)

func TestRemoteRobotsGold(t *testing.T) {
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
					ModelFilePath: "../../components/arm/fake/fake_model.json",
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

	localConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"foo:remoteArm"},
			},
			{
				Name:  "arm2",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"bar:remoteArm"},
			},
		},
		Services: []resource.Config{},
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
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("main"))

	// assert all of remote1's resources exist on main but none of remote2's
	rdktestutils.VerifySameResourceNames(
		t,
		r.ResourceNames(),
		[]resource.Name{
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
			arm.Named("arm1"),
			arm.Named("foo:remoteArm"),
			motion.Named("foo:builtin"),
			sensors.Named("foo:builtin"),
		},
	)

	// start remote2's web service
	err = remote2.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	mainPartAndFooAndBarResources := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("arm1"),
		arm.Named("arm2"),
		arm.Named("foo:remoteArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		arm.Named("bar:remoteArm"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), mainPartAndFooAndBarResources)
	})
	test.That(t, remote2.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(),
			[]resource.Name{
				motion.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
				arm.Named("arm1"),
				arm.Named("foo:remoteArm"),
				motion.Named("foo:builtin"),
				sensors.Named("foo:builtin"),
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
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), mainPartAndFooAndBarResources)
	})
}

func TestRemoteRobotsUpdate(t *testing.T) {
	// The test tests that the robot is able to update when multiple remote robot
	// updates happen at the same time.
	logger := logging.NewTestLogger(t)
	remoteConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
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
				Address: addr1,
			},
			{
				Name:    "bar",
				Address: addr1,
			},
			{
				Name:    "hello",
				Address: addr1,
			},
			{
				Name:    "world",
				Address: addr1,
			},
		},
	}
	r := setupLocalRobot(t, ctx, localConfig, logger.Sublogger("local"))

	expectedSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("foo:arm1"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		arm.Named("bar:arm1"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
		arm.Named("hello:arm1"),
		motion.Named("hello:builtin"),
		sensors.Named("hello:builtin"),
		arm.Named("world:arm1"),
		motion.Named("world:builtin"),
		sensors.Named("world:builtin"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})
	test.That(t, remote.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(),
			[]resource.Name{
				motion.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
			},
		)
	})
}

func TestInferRemoteRobotDependencyConnectAtStartup(t *testing.T) {
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
					ModelFilePath: "../../components/arm/fake/fake_model.json",
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
					ModelFilePath: "../../components/arm/fake/fake_model.json",
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
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("arm1"),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
	}

	rdktestutils.VerifySameResourceNames(t, r.ResourceNames(), expectedSet)
	test.That(t, foo.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(),
			[]resource.Name{
				motion.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
			},
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
					ModelFilePath: "../../components/arm/fake/fake_model.json",
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
					ModelFilePath: "../../components/arm/fake/fake_model.json",
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
		[]resource.Name{
			motion.Named(resource.DefaultServiceName),
			sensors.Named(resource.DefaultServiceName),
		},
	)
	err := foo.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	expectedSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("arm1"),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
	}
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})
	test.That(t, foo.Close(context.Background()), test.ShouldBeNil)

	// wait for local_robot to detect that the remote is now offline
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(),
			[]resource.Name{
				motion.Named(resource.DefaultServiceName),
				sensors.Named(resource.DefaultServiceName),
			},
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
					ModelFilePath: "../../components/arm/fake/fake_model.json",
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
					ModelFilePath: "../../components/arm/fake/fake_model.json",
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

	expectedSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		arm.Named("bar:pieceArm"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
	}

	rdktestutils.VerifySameResourceNames(t, r.ResourceNames(), expectedSet)

	// we expect the robot to correctly detect the ambiguous dependency and not build the resource
	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 150, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), expectedSet)
	})

	// now reconfig with a fully qualified name
	reConfig := &config.Config{
		Components: []resource.Config{
			{
				Name:  "arm1",
				Model: resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{
					ModelFilePath: "../../components/arm/fake/fake_model.json",
				},
				API:       arm.API,
				DependsOn: []string{"foo:pieceArm"},
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
	r.Reconfigure(ctx, reConfig)

	finalSet := []resource.Name{
		motion.Named(resource.DefaultServiceName),
		sensors.Named(resource.DefaultServiceName),
		arm.Named("foo:pieceArm"),
		motion.Named("foo:builtin"),
		sensors.Named("foo:builtin"),
		arm.Named("bar:pieceArm"),
		motion.Named("bar:builtin"),
		sensors.Named("bar:builtin"),
		arm.Named("arm1"),
	}

	testutils.WaitForAssertionWithSleep(t, time.Millisecond*100, 300, func(tb testing.TB) {
		rdktestutils.VerifySameResourceNames(tb, r.ResourceNames(), finalSet)
	})
}
