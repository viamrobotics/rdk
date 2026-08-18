package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/robottestutils"
)

var remoteMultiAPIModel = resource.DefaultModelFamily.WithModel("multiapi_remote")

// TestMultiAPIResourceOverRemote verifies that a composite (multi-API) resource on a remote robot is
// reachable through the main robot under each of its APIs (with the remote prefix). A remote
// advertises one such resource as two names sharing a bare name; the main robot surfaces both and
// each routes to the one instance on the remote.
func TestMultiAPIResourceOverRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	resource.RegisterMultiAPI(
		[]resource.API{sensor.API, generic.API},
		remoteMultiAPIModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				return &multiAPIThing{Named: conf.ResourceName().AsNamed()}, nil
			},
		},
	)
	defer func() {
		resource.Deregister(sensor.API, remoteMultiAPIModel)
		resource.Deregister(generic.API, remoteMultiAPIModel)
	}()

	// Remote robot serving the composite resource (api-less config).
	remoteCfg := &config.Config{
		Components: []resource.Config{{Name: "combo", Model: remoteMultiAPIModel}},
	}
	test.That(t, remoteCfg.Ensure(false, logger), test.ShouldBeNil)
	remote := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("remote"))

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, remote.StartWeb(ctx, options), test.ShouldBeNil)

	// Main robot connecting to the remote.
	mainCfg := &config.Config{
		Remotes: []config.Remote{{Name: "rem", Address: addr}},
	}
	main := setupLocalRobot(t, ctx, mainCfg, logger.Sublogger("main"))

	// The composite surfaces under both APIs with the remote prefix.
	names := main.ResourceNames()
	test.That(t, names, test.ShouldContain, sensor.Named("rem:combo"))
	test.That(t, names, test.ShouldContain, generic.Named("rem:combo"))

	// Usable under the sensor API through the remote.
	s, err := sensor.FromProvider(main, "rem:combo")
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 42)

	// And under the generic API — routing to the one instance on the remote.
	g, err := generic.FromProvider(main, "rem:combo")
	test.That(t, err, test.ShouldBeNil)
	resp, err := g.DoCommand(ctx, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["did"], test.ShouldEqual, "it")
}
