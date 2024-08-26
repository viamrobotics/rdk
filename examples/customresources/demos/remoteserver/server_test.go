//go:build !no_tflite

package main_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	_ "go.viam.com/rdk/examples/customresources/models/mygizmo"
	"go.viam.com/rdk/logging"
	robotimpl "go.viam.com/rdk/robot/impl"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

func TestGizmo(t *testing.T) {
	// This test sets up three robots as a chain of remotes: MainPart -> A -> B. For setup, the test
	// brings up the robots in reverse order. Remote "B" constructs a component using a custom
	// "Gizmo" API + model. The test then asserts a connection to the `MainPart` can get a handle on
	// a remote resource provided by "B". Additionally, remote "A" is not aware of the gizmo API nor
	// the gizmo model.
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Create remote B. Loop to ensure we find an available port.
	var remoteAddrB string
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		port, err := goutils.TryReserveRandomPort()
		test.That(t, err, test.ShouldBeNil)
		remoteAddrB = fmt.Sprintf("localhost:%d", port)
		test.That(t, err, test.ShouldBeNil)

		cfgServer, err := config.Read(ctx, utils.ResolveFile("./examples/customresources/demos/remoteserver/remote.json"), logger)
		test.That(t, err, test.ShouldBeNil)
		remoteB, err := robotimpl.New(ctx, cfgServer, logger.Sublogger("remoteB"))
		test.That(t, err, test.ShouldBeNil)
		options := weboptions.New()
		options.Network.BindAddress = remoteAddrB

		err = remoteB.StartWeb(ctx, options)
		if err != nil && strings.Contains(err.Error(), "address already in use") {
			logger.Infow("Port in use. Restarting on new port.", "port", port, "err", err)
			test.That(t, remoteB.Close(context.Background()), test.ShouldBeNil)
			continue
		}
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, remoteB.Close(context.Background()), test.ShouldBeNil)
		}()
		break
	}

	// Create remote A. Loop to ensure we find an available port.
	var remoteAddrA string
	success := false
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		// The process executing this test has loaded the "gizmo" API + model into a global registry
		// object. We want this intermediate remote to be unaware of the custom gizmo resource. We
		// start up a separate viam-server process to achieve this.
		port, err := goutils.TryReserveRandomPort()
		test.That(t, err, test.ShouldBeNil)
		remoteAddrA = fmt.Sprintf("localhost:%d", port)

		tmpConf, err := os.CreateTemp(t.TempDir(), "*.json")
		test.That(t, err, test.ShouldBeNil)
		_, err = tmpConf.WriteString(fmt.Sprintf(
			`{"network":{"bind_address":"%s"},"remotes":[{"address":"%s","name":"robot1"}]}`,
			remoteAddrA, remoteAddrB))
		test.That(t, err, test.ShouldBeNil)
		err = tmpConf.Sync()
		test.That(t, err, test.ShouldBeNil)

		processLogger, logObserver := logging.NewObservedTestLogger(t)
		pmgr := pexec.NewProcessManager(processLogger.Sublogger("remoteA"))
		pCfg := pexec.ProcessConfig{
			ID:      "Intermediate",
			Name:    "go",
			Args:    []string{"run", utils.ResolveFile("./web/cmd/server/main.go"), "-config", tmpConf.Name()},
			CWD:     "",
			OneShot: false,
			Log:     true,
		}
		_, err = pmgr.AddProcessFromConfig(context.Background(), pCfg)
		test.That(t, err, test.ShouldBeNil)
		err = pmgr.Start(context.Background())
		test.That(t, err, test.ShouldBeNil)
		if success = robottestutils.WaitForServing(logObserver, port); success {
			defer func() {
				test.That(t, pmgr.Stop(), test.ShouldBeNil)
			}()
			break
		}
		logger.Infow("Port in use. Restarting on new port.", "port", port, "err", err)
		pmgr.Stop()
		continue
	}
	test.That(t, success, test.ShouldBeTrue)

	// Create the MainPart. Note we will access this directly and therefore it does not need to
	// start a gRPC server.
	mainPartConfig := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "remoteA",
				Address: remoteAddrA,
			},
		},
	}
	mainPart, err := robotimpl.New(ctx, mainPartConfig, logger.Sublogger("mainPart.client"))
	defer func() {
		test.That(t, mainPart.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)

	// remotes can take a few seconds to show up, so we wait for the resource
	var res interface{}
	testutils.WaitForAssertionWithSleep(t, time.Second, 120, func(tb testing.TB) {
		res, err = mainPart.ResourceByName(gizmoapi.Named("gizmo1"))
		test.That(tb, err, test.ShouldBeNil)
	})

	gizmo1, ok := res.(gizmoapi.Gizmo)
	test.That(t, ok, test.ShouldBeTrue)
	_, err = gizmo1.DoOne(context.Background(), "hello")
	test.That(t, err, test.ShouldBeNil)
	_, err = gizmo1.DoOneClientStream(context.Background(), []string{"hello", "arg1", "foo"})
	test.That(t, err, test.ShouldBeNil)
	_, err = gizmo1.DoOneServerStream(context.Background(), "hello")
	test.That(t, err, test.ShouldBeNil)
	_, err = gizmo1.DoOneBiDiStream(context.Background(), []string{"hello", "arg1", "foo"})
	test.That(t, err, test.ShouldBeNil)
	_, err = gizmo1.DoOneBiDiStream(context.Background(), []string{"arg1", "arg1", "arg1"})
	test.That(t, err, test.ShouldBeNil)

	_, err = mainPart.ResourceByName(gizmoapi.Named("remoteA:robot1:gizmo1"))
	test.That(t, err, test.ShouldBeNil)

	_, err = mainPart.ResourceByName(gizmoapi.Named("remoteA:gizmo1"))
	test.That(t, err, test.ShouldNotBeNil)
}
