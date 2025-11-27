package module

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/testutils/inject"
)

func TestModularMain(t *testing.T) {
	logger := logging.NewTestLogger(t)
	// check tcp and unix

	robotServerListener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectRobot := &inject.Robot{
		ResourceNamesFunc:  func() []resource.Name { return []resource.Name{} },
		ResourceByNameFunc: func(n resource.Name) (resource.Resource, error) { return nil, nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
		LoggerFunc:          func() logging.Logger { return logger },
	}

	robotpb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	var wg sync.WaitGroup
	wg.Go(func() { gServer.Serve(robotServerListener) })

	p, err := goutils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	modAddr := fmt.Sprintf(":%d", p)
	wg.Go(func() {
		modErr := modMain(modAddr)(context.Background(), nil, logger)
		test.That(t, modErr, test.ShouldBeError, context.Canceled)
	})
	conn, err := grpc.DialContext(context.Background(), //nolint:staticcheck
		modAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), //nolint:staticcheck
	)
	test.That(t, err, test.ShouldBeNil)
	mod := pb.NewModuleServiceClient(conn)
	_, err = mod.Ready(context.Background(), &pb.ReadyRequest{ParentAddress: robotServerListener.Addr().String()})
	test.That(t, err, test.ShouldBeNil)

	gServer.Stop()
	wg.Wait()
}
