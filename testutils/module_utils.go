package testutils

import (
	"context"
	"net"
	"testing"

	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

type mockRobotService struct {
	robotpb.UnimplementedRobotServiceServer
}

// Log is a no-op for the mockRobotService.
func (ms *mockRobotService) Log(ctx context.Context, req *robotpb.LogRequest) (*robotpb.LogResponse, error) {
	return &robotpb.LogResponse{}, nil
}

// MakeRobotForModuleLogging creates and starts an RPC server that can respond
// to `LogRequest`s from modules and listens at parentAddr.
func MakeRobotForModuleLogging(t *testing.T, parentAddr string) rpc.Server {
	logger := logging.NewTestLogger(t)
	prot := "unix"
	if utils.TCPRegex.MatchString(parentAddr) {
		prot = "tcp"
	}
	listener, err := net.Listen(prot, parentAddr)
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	robotService := &mockRobotService{}
	test.That(t, rpcServer.RegisterServiceServer(
		context.Background(),
		&robotpb.RobotService_ServiceDesc,
		robotService,
		robotpb.RegisterRobotServiceHandlerFromEndpoint,
	), test.ShouldBeNil)

	go func() {
		test.That(t, rpcServer.Serve(listener), test.ShouldBeNil)
	}()
	t.Cleanup(func() {
		test.That(t, rpcServer.Stop(), test.ShouldBeNil)
	})
	return rpcServer
}
