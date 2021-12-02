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
	"go.viam.com/core/grpc"
	"go.viam.com/core/grpc/client"
	"go.viam.com/core/metadata/service"
	commonpb "go.viam.com/core/proto/api/common/v1"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/testutils/inject"

	rpcclient "go.viam.com/utils/rpc/client"

	_ "go.viam.com/core/component/arm/register"
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

	// make sure we get something back
	test.That(t, err, test.ShouldBeNil)
	client, err := client.NewClient(context.Background(), "localhost:8080", logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, client.ResourceNames(), test.ShouldResemble, resources)

	// try to start another server
	err = svc.Start(context.Background(), NewOptions())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already started")

	// try to close server
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
	test.That(t, client.ResourceNames(), test.ShouldResemble, resources)

	// try to close server
	err = svc.Close()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)
}

func TestWebUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx, robot := setupRobotCtx()

	svc, err := New(ctx, robot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)

	err = svc.Start(ctx, NewOptions())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldNotBeNil)

	// make sure we get something back
	addr := "localhost:8080"
	arm1 := "arm1"
	c, err := client.NewClient(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c.ResourceNames(), test.ShouldResemble, resources)

	// add arm to robot and then update
	injectArm := &inject.Arm{}
	pos := &commonpb.Pose{X: 1, Y: 2, Z: 3}
	injectArm.CurrentPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos, nil
	}
	rs := map[resource.Name]interface{}{arm.Named(arm1): injectArm}
	updateable := svc.(resource.Updateable)
	err = updateable.Update(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)

	conn, err := grpc.Dial(context.Background(), addr, rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	aClient := arm.NewClientFromConn(conn, arm1, logger)
	position, err := aClient.CurrentPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos)

	// try to close server
	test.That(t, svc.Close(), test.ShouldBeNil)
	test.That(t, svc.(*webService).cancelFunc, test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)

	// now start it with the arm already in it
	ctx, robot2 := setupRobotCtx()
	robot2.(*inject.Robot).ResourceNamesFunc = func() []resource.Name { return append(resources, arm.Named(arm1)) }
	robot2.(*inject.Robot).ResourceByNameFunc = func(name resource.Name) (interface{}, bool) { return injectArm, true }

	svc2, err := New(ctx, robot2, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc2.(*webService).cancelFunc, test.ShouldBeNil)

	err = svc2.Start(ctx, NewOptions())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc2.(*webService).cancelFunc, test.ShouldNotBeNil)

	c2, err := client.NewClient(context.Background(), addr, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c2.ResourceNames(), test.ShouldResemble, resources)
	conn, err = grpc.Dial(context.Background(), addr, rpcclient.DialOptions{Insecure: true}, logger)
	test.That(t, err, test.ShouldBeNil)
	aClient2 := arm.NewClientFromConn(conn, arm1, logger)
	test.That(t, err, test.ShouldBeNil)
	position, err = aClient2.CurrentPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos)

	// add a second arm
	arm2 := "arm2"
	injectArm2 := &inject.Arm{}
	pos2 := &commonpb.Pose{X: 2, Y: 3, Z: 4}
	injectArm2.CurrentPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pos2, nil
	}
	rs[arm.Named(arm2)] = injectArm2
	updateable = svc2.(resource.Updateable)
	err = updateable.Update(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)

	position, err = aClient2.CurrentPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos)

	aClient3 := arm.NewClientFromConn(conn, arm2, logger)
	test.That(t, err, test.ShouldBeNil)
	position, err = aClient3.CurrentPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, position, test.ShouldResemble, pos2)

	// try to close server
	test.That(t, svc2.Close(), test.ShouldBeNil)
	test.That(t, svc2.(*webService).cancelFunc, test.ShouldBeNil)
	test.That(t, conn.Close(), test.ShouldBeNil)
}

func setupRobotCtx() (context.Context, robot.Robot) {
	injectRobot := &inject.Robot{}
	injectRobot.ConfigFunc = func(ctx context.Context) (*config.Config, error) { return nil, nil }
	injectRobot.CameraNamesFunc = func() []string { return []string{} }
	injectRobot.LidarNamesFunc = func() []string { return []string{} }
	injectRobot.ResourceNamesFunc = func() []resource.Name { return resources }
	injectRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) { return name, false }
	injectRobot.StatusFunc = func(ctx context.Context) (*pb.Status, error) { return &pb.Status{}, nil }

	injectMetadata := &inject.Metadata{}
	injectMetadata.AllFunc = func() []resource.Name { return resources }
	ctx := service.ContextWithService(context.Background(), injectMetadata)

	return ctx, injectRobot
}
