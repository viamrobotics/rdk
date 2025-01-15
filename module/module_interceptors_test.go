package module_test

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/component/generic/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
)

func TestOpID(t *testing.T) {
	ctx := context.Background()

	if runtime.GOARCH == "arm" {
		t.Skip("skipping on 32-bit ARM -- subprocess build warnings cause failure")
	}
	logger, logObserver := logging.NewObservedTestLogger(t)

	var port int
	var success bool
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		cfgFilename, p, err := makeConfig(t, logger)
		port = p
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, os.Remove(cfgFilename), test.ShouldBeNil)
		}()

		server := robottestutils.ServerAsSeparateProcess(t, cfgFilename, logger)
		err = server.Start(context.Background())
		test.That(t, err, test.ShouldBeNil)

		if success = robottestutils.WaitForServing(logObserver, port); success {
			defer func() {
				test.That(t, server.Stop(), test.ShouldBeNil)
			}()
			break
		}
		logger.Infow("Port in use. Restarting on new port.", "port", port, "err", err)
		server.Stop()
		continue
	}
	test.That(t, success, test.ShouldBeTrue)

	conn, err := robottestutils.Connect(port)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()
	rc := robotpb.NewRobotServiceClient(conn)
	gc := genericpb.NewGenericServiceClient(conn)

	opIDOutgoing := uuid.New().String()
	var opIDIncoming string
	md := metadata.New(map[string]string{"opid": opIDOutgoing})
	mdCtx := metadata.NewOutgoingContext(ctx, md)

	// Wait for generic helper to build on machine (machine to report a state of "running.")
	for {
		mStatus, err := rc.GetMachineStatus(ctx, &robotpb.GetMachineStatusRequest{})
		test.That(t, err, test.ShouldBeNil)
		if mStatus.State == robotpb.GetMachineStatusResponse_STATE_RUNNING {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Do this twice, once with no opID set, and a second with a set opID.
	for name, cCtx := range map[string]context.Context{"default context": ctx, "context with opid set": mdCtx} {
		t.Run(name, func(t *testing.T) {
			syncChan := make(chan string)
			// in the background, run a operation that sleeps for one second, and capture it's header
			go func() {
				cmd, err := structpb.NewStruct(map[string]interface{}{"command": "sleep"})
				test.That(t, err, test.ShouldBeNil)

				var hdr metadata.MD
				_, err = gc.DoCommand(cCtx, &commonpb.DoCommandRequest{Name: "helper1", Command: cmd}, grpc.Header(&hdr))
				test.That(t, err, test.ShouldBeNil)

				test.That(t, hdr["opid"], test.ShouldHaveLength, 1)
				origOpID, err := uuid.Parse(hdr["opid"][0])
				test.That(t, err, test.ShouldBeNil)
				syncChan <- origOpID.String()
			}()

			// directly get the operations list from the parent server, naively waiting for it to be non-zero
			var parentOpList, modOpList []string
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				resp, err := rc.GetOperations(ctx, &robotpb.GetOperationsRequest{})
				test.That(tb, err, test.ShouldBeNil)
				opList := resp.GetOperations()
				test.That(tb, len(opList), test.ShouldBeGreaterThan, 0)
				for _, op := range opList {
					parentOpList = append(parentOpList, op.Id)
				}
			})

			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				// as soon as we see the op in the parent, check the operations in the module
				cmd, err := structpb.NewStruct(map[string]interface{}{"command": "get_ops"})
				test.That(tb, err, test.ShouldBeNil)
				resp, err := gc.DoCommand(ctx, &commonpb.DoCommandRequest{Name: "helper1", Command: cmd})
				test.That(tb, err, test.ShouldBeNil)
				ret, ok := resp.GetResult().AsMap()["ops"]
				test.That(tb, ok, test.ShouldBeTrue)
				retList, ok := ret.([]interface{})
				test.That(tb, ok, test.ShouldBeTrue)
				test.That(tb, len(retList), test.ShouldBeGreaterThan, 1)
				for _, v := range retList {
					val, ok := v.(string)
					test.That(tb, ok, test.ShouldBeTrue)
					modOpList = append(modOpList, val)
				}
				test.That(tb, len(modOpList), test.ShouldBeGreaterThan, 1)
			})

			// wait for the original call to sleep and parse its header for the opID
			commandOpID := <-syncChan

			// then make sure the initial opID showed up in both parent and module correctly
			test.That(t, commandOpID, test.ShouldBeIn, parentOpList)
			test.That(t, commandOpID, test.ShouldBeIn, modOpList)
			opIDIncoming = commandOpID

			// lastly, and only for the second iteration, make sure the intentionally set, outgoing opID was used
			if name != "default context" {
				test.That(t, opIDIncoming, test.ShouldEqual, opIDOutgoing)
			}
		})
	}
}

func TestModuleClientTimeoutInterceptor(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Precompile module to avoid timeout issues when building takes too long.
	modPath := rtestutils.BuildTempModule(t, "module/testmodule")
	cfg := &config.Config{
		Modules: []config.Module{{
			Name:    "test",
			ExePath: modPath,
		}},
		Components: []resource.Config{{
			API:   generic.API,
			Model: resource.NewModel("rdk", "test", "helper"),
			Name:  "helper1",
		}},
	}
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r.Close(ctx), test.ShouldBeNil)
	}()

	helper1, err := r.ResourceByName(generic.Named("helper1"))
	test.That(t, err, test.ShouldBeNil)

	// Artificially set default method timeout to have timed out in the past.
	origDefaultMethodTimeout := rdkgrpc.DefaultMethodTimeout
	rdkgrpc.DefaultMethodTimeout = -time.Nanosecond
	defer func() {
		rdkgrpc.DefaultMethodTimeout = origDefaultMethodTimeout
	}()

	t.Run("client respects default timeout", func(t *testing.T) {
		_, err = helper1.DoCommand(ctx, map[string]interface{}{"command": "echo"})

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldResemble,
			"rpc error: code = DeadlineExceeded desc = context deadline exceeded")
	})
	t.Run("deadline not overwritten", func(t *testing.T) {
		ctxWithDeadline, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		_, err = helper1.DoCommand(ctxWithDeadline, map[string]interface{}{"command": "echo"})
		test.That(t, err, test.ShouldBeNil)
	})
}

func makeConfig(t *testing.T, logger logging.Logger) (string, int, error) {
	// Precompile module to avoid timeout issues when building takes too long.
	modPath := rtestutils.BuildTempModule(t, "module/testmodule")

	port, err := goutils.TryReserveRandomPort()
	if err != nil {
		return "", 0, err
	}

	cfg := config.Config{
		Modules: []config.Module{{
			Name:    "TestModule",
			ExePath: modPath,
		}},
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				BindAddress: fmt.Sprintf("localhost:%d", port),
			},
		},
		Components: []resource.Config{{
			API:   generic.API,
			Model: resource.NewModel("rdk", "test", "helper"),
			Name:  "helper1",
		}},
	}
	cfgFilename, err := robottestutils.MakeTempConfig(t, &cfg, logger)
	return cfgFilename, port, err
}
