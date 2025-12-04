package module

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"testing"

	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func TestModularMain(t *testing.T) {
	// This test tests that ModularMain exits with a context cancelled if connection to
	// the parent robot server is lost.
	// Since ModularMain takes in cmd-line args and hijacks signal handling, we test a
	// private function that contains most of the main logic in ModularMain.

	for _, tc := range []struct {
		TestName string
		UdsMode  bool
	}{
		{"uds", true},
		{"tcp", false},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			t.Parallel()
			logger := logging.NewTestLogger(t)

			var (
				robotServerListener net.Listener
				err                 error
			)
			if tc.UdsMode {
				parentAddr, err := CreateSocketAddress(t.TempDir(), utils.RandomAlphaString(5))
				test.That(t, err, test.ShouldBeNil)
				robotServerListener, err = net.Listen("unix", parentAddr)
				test.That(t, err, test.ShouldBeNil)
			} else {
				robotServerListener, err = net.Listen("tcp", "127.0.0.1:0")
				test.That(t, err, test.ShouldBeNil)
			}
			injectRobot := &inject.Robot{
				ResourceNamesFunc: func() []resource.Name { return []resource.Name{} },
				MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
					return robot.MachineStatus{State: robot.StateRunning}, nil
				},
				ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
			}
			gServer := grpc.NewServer()
			robotpb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
			var wg sync.WaitGroup
			wg.Go(func() { gServer.Serve(robotServerListener) })

			var (
				modAddr string
				mod     *Module
			)
			modCtx, modCancel := context.WithCancel(context.Background())
			defer modCancel()

			// if port is taken, retry starting the module server a few times
			for range 10 {
				var port int
				if tc.UdsMode {
					modAddr, err = CreateSocketAddress(t.TempDir(), utils.RandomAlphaString(5))
					test.That(t, err, test.ShouldBeNil)
					modAddr, err = rutils.CleanWindowsSocketPath(runtime.GOOS, modAddr)
					test.That(t, err, test.ShouldBeNil)
				} else {
					port, err = utils.TryReserveRandomPort()
					test.That(t, err, test.ShouldBeNil)
					modAddr = fmt.Sprintf(":%d", port)
				}

				mod, err = moduleStart(modAddr)(modCtx, nil, modCancel, logger)
				if err != nil && strings.Contains(err.Error(), "address already in use") {
					logger.Infow("port in use; restarting on new port", "port", port, "err", err)
					continue
				}
				test.That(t, err, test.ShouldBeNil)
				defer mod.Close(context.Background())
				break
			}
			if tc.UdsMode {
				modAddr = "unix:" + modAddr
			}

			conn, err := grpc.Dial( //nolint:staticcheck
				modAddr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock(), //nolint:staticcheck
			)
			test.That(t, err, test.ShouldBeNil)
			modClient := pb.NewModuleServiceClient(conn)

			// This test depends on the module server not returning a response for Ready until its parent connection has
			// been established.
			_, err = modClient.Ready(context.Background(), &pb.ReadyRequest{ParentAddress: robotServerListener.Addr().String()})
			test.That(t, err, test.ShouldBeNil)

			gServer.Stop()
			wg.Wait()
			<-modCtx.Done()

			test.That(t, modCtx.Err(), test.ShouldBeError, context.Canceled)
		})
	}
}
