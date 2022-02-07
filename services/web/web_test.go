package web

import (
	"context"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/metadata/service"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils/inject"
)

var resources = []resource.Name{resource.NewName(resource.Namespace("acme"), resource.ResourceTypeComponent, arm.SubtypeName, "arm1")}

func TestWebStart(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

	svc, err := New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)

	err = svc.Start(ctx, NewOptions())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldNotBeNil)

	client, err := client.New(context.Background(), "localhost:8080", logger, client.WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, client.ResourceNames(), test.ShouldResemble, resources)

	err = svc.Start(context.Background(), NewOptions())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already started")

	err = utils.TryClose(context.Background(), svc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)
}

func TestWebStartOptions(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

	svc, err := New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := NewOptions()
	addr := fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldNotBeNil)

	client, err := client.New(context.Background(), addr, logger, client.WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, client.ResourceNames(), test.ShouldResemble, resources)

	err = utils.TryClose(context.Background(), svc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)
}

func TestWebWithAuth(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

	for _, tc := range []struct {
		Case        string
		Managed     bool
		EntityNames []string
	}{
		{Case: "unmanaged and default host"},
		{Case: "unmanaged and specific host", EntityNames: []string{"something-different", "something-really-different"}},
		{Case: "managed and default host", Managed: true},
		{Case: "managed and specific host", Managed: true, EntityNames: []string{"something-different", "something-really-different"}},
	} {
		t.Run(tc.Case, func(t *testing.T) {
			svc, err := New(ctx, injectRobot, config.Service{}, logger)
			test.That(t, err, test.ShouldBeNil)

			port, err := utils.TryReserveRandomPort()
			test.That(t, err, test.ShouldBeNil)
			options := NewOptions()
			addr := fmt.Sprintf("localhost:%d", port)
			options.Network.BindAddress = addr
			options.Managed = tc.Managed
			options.FQDNs = tc.EntityNames
			apiKey := "sosecret"
			options.Auth.Handlers = []config.AuthHandlerConfig{
				{
					Type: rpc.CredentialsTypeAPIKey,
					Config: config.AttributeMap{
						"key": apiKey,
					},
				},
			}

			err = svc.Start(ctx, options)
			test.That(t, err, test.ShouldBeNil)

			_, err = client.New(context.Background(), addr, logger, client.WithDialOptions(rpc.WithInsecure()))
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "authentication required")

			if tc.Managed {
				_, err = client.New(context.Background(), addr, logger, client.WithDialOptions(
					rpc.WithInsecure(),
					rpc.WithCredentials(rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey,
					}),
				))
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, "invalid credentials")

				entityNames := tc.EntityNames
				if len(entityNames) == 0 {
					entityNames = []string{DefaultFQDN}
				}
				for _, entityName := range entityNames {
					t.Run(entityName, func(t *testing.T) {
						c, err := client.New(context.Background(), addr, logger, client.WithDialOptions(
							rpc.WithInsecure(),
							rpc.WithEntityCredentials(entityName, rpc.Credentials{
								Type:    rpc.CredentialsTypeAPIKey,
								Payload: apiKey,
							}),
						))
						test.That(t, err, test.ShouldBeNil)
						test.That(t, c.ResourceNames(), test.ShouldResemble, resources)
					})
				}
			} else {
				c, err := client.New(context.Background(), addr, logger, client.WithDialOptions(
					rpc.WithInsecure(),
					rpc.WithCredentials(rpc.Credentials{
						Type:    rpc.CredentialsTypeAPIKey,
						Payload: apiKey,
					}),
				))
				test.That(t, err, test.ShouldBeNil)
				test.That(t, c.ResourceNames(), test.ShouldResemble, resources)
			}

			err = utils.TryClose(context.Background(), svc)
			test.That(t, err, test.ShouldBeNil)
		})
	}
}

func TestWebWithBadAuthHandlers(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

	svc, err := New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := NewOptions()
	addr := fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr
	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: "unknown",
		},
	}

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "do not know how")
	test.That(t, err.Error(), test.ShouldContainSubstring, "unknown")
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)

	svc, err = New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	port, err = utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options = NewOptions()
	addr = fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr
	options.Auth.Handlers = []config.AuthHandlerConfig{
		{
			Type: rpc.CredentialsTypeAPIKey,
		},
	}

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "non-empty")
	test.That(t, err.Error(), test.ShouldContainSubstring, "api-key")
	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
}

func TestWebUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, robot := setupRobotCtx()

	svc, err := New(ctx, robot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)

	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	options := NewOptions()
	addr := fmt.Sprintf("localhost:%d", port)
	options.Network.BindAddress = addr
	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldNotBeNil)

	arm1 := "arm1"
	c, err := client.New(context.Background(), addr, logger, client.WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c.ResourceNames(), test.ShouldResemble, resources)

	// add arm to robot and then update
	injectArm := &inject.Arm{}
	pos := &commonpb.Pose{X: 1, Y: 2, Z: 3}
	injectArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos, nil
	}
	rs := map[resource.Name]interface{}{arm.Named(arm1): injectArm}
	updateable, ok := svc.(resource.Updateable)
	test.That(t, ok, test.ShouldBeTrue)
	err = updateable.Update(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)

	conn, err := rgrpc.Dial(context.Background(), addr, logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	aClient := arm.NewClientFromConn(context.Background(), conn, arm1, logger)
	position, err := aClient.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos)

	test.That(t, utils.TryClose(context.Background(), svc), test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)

	// now start it with the arm already in it
	ctx, robot2 := setupRobotCtx()
	robot2.(*inject.Robot).ResourceNamesFunc = func() []resource.Name { return append(resources, arm.Named(arm1)) }
	robot2.(*inject.Robot).ResourceByNameFunc = func(name resource.Name) (interface{}, bool) { return injectArm, true }

	svc2, err := New(ctx, robot2, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc2.(*webService).cancelFunc, test.ShouldBeNil)

	err = svc2.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc2.(*webService).cancelFunc, test.ShouldNotBeNil)

	c2, err := client.New(context.Background(), addr, logger, client.WithDialOptions(rpc.WithInsecure()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c2.ResourceNames(), test.ShouldResemble, resources)
	conn, err = rgrpc.Dial(context.Background(), addr, logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	aClient2 := arm.NewClientFromConn(context.Background(), conn, arm1, logger)
	test.That(t, err, test.ShouldBeNil)
	position, err = aClient2.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos)

	// add a second arm
	arm2 := "arm2"
	injectArm2 := &inject.Arm{}
	pos2 := &commonpb.Pose{X: 2, Y: 3, Z: 4}
	injectArm2.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos2, nil
	}
	rs[arm.Named(arm2)] = injectArm2
	updateable, ok = svc2.(resource.Updateable)
	test.That(t, ok, test.ShouldBeTrue)
	err = updateable.Update(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)

	position, err = aClient2.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos)

	aClient3 := arm.NewClientFromConn(context.Background(), conn, arm2, logger)
	test.That(t, err, test.ShouldBeNil)
	position, err = aClient3.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos2)

	test.That(t, utils.TryClose(context.Background(), svc2), test.ShouldBeNil)
	test.That(t, svc2.(*webService).cancelFunc, test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func setupRobotCtx() (context.Context, robot.Robot) {
	injectRobot := &inject.Robot{}
	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) { return &config.Config{}, nil }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return resources }
	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) { return name, false }
	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) { return &pb.Status{}, nil }

	injectMetadata := &inject.Metadata{}
	injectMetadata.AllFunc = func() []resource.Name { return resources }
	ctx := service.ContextWithService(context.Background(), injectMetadata)

	return ctx, injectRobot
}
