package shell_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var testSvcName1 = shell.Named("shell1")

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectShell := &inject.ShellService{}
	ssMap := map[resource.Name]resource.Resource{
		testSvcName1: injectShell,
	}
	svc, err := subtype.New(shell.Subtype, ssMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype, ok := registry.ResourceSubtypeLookup(shell.Subtype)
	test.That(t, ok, test.ShouldBeTrue)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// working client
	t.Run("shell client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client, err := shell.NewClientFromConn(context.Background(), conn, testSvcName1, logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		injectShell.DoCommandFunc = testutils.EchoFunc
		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}
