package module_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	genericpb "go.viam.com/api/component/generic/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

func TestOpID(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfgFilename, port, err := makeConfig()
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, os.Remove(cfgFilename), test.ShouldBeNil)
	}()
	server := pexec.NewManagedProcess(pexec.ProcessConfig{
		Name: "bash",
		Args: []string{"-c", "make server && exec bin/`uname`-`uname -m`/server -config " + cfgFilename},
		CWD:  utils.ResolveFile("./"),
		Log:  true,
	}, logger)

	err = server.Start(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, server.Stop(), test.ShouldBeNil)
	}()

	rc, gc, conn, err := connect(port)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()

	opIDOutgoing := uuid.New().String()
	var opIDIncoming string
	md := metadata.New(map[string]string{"opid": opIDOutgoing})
	mdCtx := metadata.NewOutgoingContext(ctx, md)

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

func connect(port string) (robotpb.RobotServiceClient, genericpb.GenericServiceClient, *grpc.ClientConn, error) {
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, time.Minute)
	defer cancelFunc()

	var conn *grpc.ClientConn
	conn, err := grpc.DialContext(ctxTimeout,
		"dns:///localhost:"+port,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, nil, err
	}

	rc := robotpb.NewRobotServiceClient(conn)
	gc := genericpb.NewGenericServiceClient(conn)

	return rc, gc, conn, nil
}

func makeConfig() (string, string, error) {
	// Precompile module to avoid timeout issues when building takes too long.
	builder := exec.Command("go", "build", ".")
	builder.Dir = utils.ResolveFile("module/testmodule")
	out, err := builder.CombinedOutput()
	if len(out) != 0 || err != nil {
		return "", "", multierr.Combine(err, fmt.Errorf("module build output: %s", out))
	}

	p, err := goutils.TryReserveRandomPort()
	if err != nil {
		return "", "", err
	}
	port := strconv.Itoa(p)

	cfg := config.Config{
		Modules: []config.Module{{
			Name:    "TestModule",
			ExePath: utils.ResolveFile("module/testmodule/run.sh"),
		}},
		Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{BindAddress: "localhost:" + port}},
		Components: []config.Component{{
			Namespace: resource.ResourceNamespaceRDK,
			Type:      "generic",
			Model:     resource.NewModel("rdk", "test", "helper"),
			Name:      "helper1",
		}},
	}

	output, err := json.Marshal(cfg)
	if err != nil {
		return "", "", err
	}
	file, err := os.CreateTemp("", "viam-test-config-*")
	if err != nil {
		return "", "", err
	}
	cfgFilename := file.Name()
	_, err = file.Write(output)
	if err != nil {
		return "", "", err
	}
	return cfgFilename, port, file.Close()
}
