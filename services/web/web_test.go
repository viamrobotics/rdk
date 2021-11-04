package web

import (
	"context"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/grpc/client"
	"go.viam.com/core/metadata/service"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/testutils/inject"
)

func TestWebStart(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, injectRobot := setupRobotCtx()

	svc, err := New(ctx, injectRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)

	err = svc.Start(ctx, NewOptions())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldNotBeNil)

	// make sure we get something back
	test.That(t, err, test.ShouldBeNil)
	client, err := client.NewClient(context.Background(), "localhost:8080", logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, client.ArmNames(), test.ShouldResemble, []string{"arm1"})

	// try to start another server
	err = svc.Start(context.Background(), NewOptions())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already started")

	// try to close to server
	err = svc.Close()
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
	options.Port = port

	addr := fmt.Sprintf("localhost:%d", port)

	err = svc.Start(ctx, options)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldNotBeNil)

	// make sure we get something back
	test.That(t, err, test.ShouldBeNil)
	client, err := client.NewClient(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, client.ArmNames(), test.ShouldResemble, []string{"arm1"})

	// try to close to server
	err = svc.Close()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)
}

func setupRobotCtx() (context.Context, robot.Robot) {
	var resources = []resource.Name{arm.Named("arm1")}
	injectRobot := &inject.Robot{}
	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) { return nil, nil }
	injectRobot.CameraNamesFunc = func() []string { return []string{} }
	injectRobot.LidarNamesFunc = func() []string { return []string{} }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return resources }
	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) { return &pb.Status{}, nil }

	injectMetadata := &inject.Metadata{}
	injectMetadata.AllFunc = func() []resource.Name { return resources }
	ctx := service.ContextWithService(context.Background(), injectMetadata)

	return ctx, injectRobot
}
