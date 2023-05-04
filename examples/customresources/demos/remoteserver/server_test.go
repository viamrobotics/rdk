package main_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	_ "go.viam.com/rdk/examples/customresources/models/mygizmo"
	robotimpl "go.viam.com/rdk/robot/impl"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/utils"
)

func TestGizmo(t *testing.T) {
	port1, err := goutils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr1 := fmt.Sprintf("localhost:%d", port1)
	test.That(t, err, test.ShouldBeNil)
	port2, err := goutils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr2 := fmt.Sprintf("localhost:%d", port2)

	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfgServer, err := config.Read(ctx, utils.ResolveFile("./examples/customresources/demos/remoteserver/remote.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	r0, err := robotimpl.New(ctx, cfgServer, logger.Named("gizmo.server"))
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r0.Close(context.Background()), test.ShouldBeNil)
	}()
	options := weboptions.New()
	options.Network.BindAddress = addr1
	err = r0.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	tmpConf, err := os.CreateTemp(t.TempDir(), "*.json")
	test.That(t, err, test.ShouldBeNil)
	_, err = tmpConf.Write([]byte(fmt.Sprintf(`{"network":{"bind_address":"%s"},"remotes":[{"address":"%s","name":"robot1"}]}`, addr2, addr1)))
	test.That(t, err, test.ShouldBeNil)
	err = tmpConf.Sync()
	test.That(t, err, test.ShouldBeNil)
	pmgr := pexec.NewProcessManager(logger.Named("process.inter"))
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
	defer func() {
		test.That(t, pmgr.Stop(), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)

	remoteConfig := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "remoteA",
				Address: addr2,
			},
		},
	}
	r2, err := robotimpl.New(ctx, remoteConfig, logger.Named("gizmo.client"))
	defer func() {
		test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)

	// remotes can take a few seconds to show up, so we wait for the resource
	var res interface{}
	testutils.WaitForAssertionWithSleep(t, time.Second, 120, func(tb testing.TB) {
		res, err = r2.ResourceByName(gizmoapi.Named("gizmo1"))
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

	_, err = r2.ResourceByName(gizmoapi.Named("remoteA:robot1:gizmo1"))
	test.That(t, err, test.ShouldBeNil)

	_, err = r2.ResourceByName(gizmoapi.Named("remoteA:gizmo1"))
	test.That(t, err, test.ShouldNotBeNil)
}
