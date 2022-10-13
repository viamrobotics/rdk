package mycomponent_test

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

	"go.viam.com/rdk/config"
	myc "go.viam.com/rdk/examples/mycomponent/component"
	robotimpl "go.viam.com/rdk/robot/impl"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/utils"
)

func TestMyComponent(t *testing.T) {
	port1, err := goutils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr1 := fmt.Sprintf("localhost:%d", port1)
	test.That(t, err, test.ShouldBeNil)
	port2, err := goutils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	addr2 := fmt.Sprintf("localhost:%d", port2)

	ctx := context.Background()
	logger := golog.NewDebugLogger("mycomponent.server")

	cfgServer, err := config.Read(ctx, utils.ResolveFile("./examples/mycomponent/server/config.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	r0, err := robotimpl.New(ctx, cfgServer, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, r0.Close(context.Background()), test.ShouldBeNil)
	}()
	options := weboptions.New()
	options.Network.BindAddress = addr1
	err = r0.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	tmpConf, err := os.CreateTemp("", "*.json")
	test.That(t, err, test.ShouldBeNil)
	_, err = tmpConf.Write([]byte(fmt.Sprintf(`{"network":{"bind_address":"%s"},"remotes":[{"address":"%s","name":"robot1"}]}`, addr2, addr1)))
	test.That(t, err, test.ShouldBeNil)
	err = tmpConf.Sync()
	test.That(t, err, test.ShouldBeNil)
	logger = golog.NewDebugLogger("process.inter")
	pmgr := pexec.NewProcessManager(logger)
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
	goutils.SelectContextOrWait(context.Background(), 30*time.Second)

	logger = golog.NewDebugLogger("mycomponent.client")
	remoteConfig := &config.Config{
		Remotes: []config.Remote{
			{
				Name:    "remoteA",
				Address: addr2,
			},
		},
	}
	r2, err := robotimpl.New(ctx, remoteConfig, logger)
	defer func() {
		test.That(t, r2.Close(context.Background()), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	res, err := r2.ResourceByName(myc.Named("comp1"))
	test.That(t, err, test.ShouldBeNil)
	comp1, ok := res.(myc.MyComponent)
	test.That(t, ok, test.ShouldBeTrue)
	_, err = comp1.DoOne(context.Background(), "hello")
	test.That(t, err, test.ShouldBeNil)
	_, err = comp1.DoOneClientStream(context.Background(), []string{"hello", "arg1", "foo"})
	test.That(t, err, test.ShouldBeNil)
	_, err = comp1.DoOneServerStream(context.Background(), "hello")
	test.That(t, err, test.ShouldBeNil)
	_, err = comp1.DoOneBiDiStream(context.Background(), []string{"hello", "arg1", "foo"})
	test.That(t, err, test.ShouldBeNil)
	_, err = comp1.DoOneBiDiStream(context.Background(), []string{"arg1", "arg1", "arg1"})
	test.That(t, err, test.ShouldBeNil)

	_, err = r2.ResourceByName(myc.Named("remoteA:robot1:comp1"))
	test.That(t, err, test.ShouldBeNil)

	_, err = r2.ResourceByName(myc.Named("remoteA:comp1"))
	test.That(t, err, test.ShouldNotBeNil)
}
