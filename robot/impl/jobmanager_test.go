package robotimpl

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.viam.com/rdk/components/arm"
	//"google.golang.org/grpc/codes"
	//"google.golang.org/grpc/status"
	//"go.viam.com/rdk/components/arm/fake"
	//"go.viam.com/rdk/components/audioinput"
	//"go.viam.com/rdk/components/base"
	//"go.viam.com/rdk/components/board"
	//"go.viam.com/rdk/components/camera"
	//"go.viam.com/rdk/components/encoder"
	//fakeencoder "go.viam.com/rdk/components/encoder/fake"
	//"go.viam.com/rdk/components/generic"
	//"go.viam.com/rdk/components/gripper"
	//"go.viam.com/rdk/components/motor"
	//fakemotor "go.viam.com/rdk/components/motor/fake"
	//"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	//"go.viam.com/rdk/components/sensor"
	"go.viam.com/test"

	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/utils"
	//"go.viam.com/utils/rpc"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/utils/testutils"
)

// tests to do:
// test that checks seconds schedule
// test that checks cron schedule
// test that checks do command
// test that checks every service (one method each)
// test that checks expected errors (?)
// test that checks that jobs can be modified/added/removed

func TestConfigJobManager(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), "data/fake_jobs.json", logger, nil)
	test.That(t, err, test.ShouldBeNil)
	logger, logs := logging.NewObservedTestLogger(t)
	robotContext := context.Background()
	setupLocalRobot(t, robotContext, cfg, logger)

	testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, logs.FilterMessage("Job triggered").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
		test.That(tb, logs.FilterMessage("Job succeeded").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 1)
	})

}

func TestJobManagerDoCommand(t *testing.T) {

	logger := logging.NewTestLogger(t)
	//channel := make(chan struct{})
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))

	//logger, logs := logging.NewObservedTestLogger(t)
	var (
		doCommandCount1 int
		doCommandCount2 int
	)
	dummyArm1 := &inject.Arm{
		DoFunc: func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			doCommandCount1++
			return map[string]any{
				"count": doCommandCount1,
			}, nil
		},
	}
	dummyArm2 := &inject.Arm{
		DoFunc: func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			doCommandCount2++
			return map[string]any{
				"count": doCommandCount2,
			}, nil
		},
	}
	resource.RegisterComponent(
		arm.API,
		model,
		resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (arm.Arm, error) {
			if conf.Name == "arm1" {
				return dummyArm1, nil
			}
			return dummyArm2, nil
		}})

	armConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%[1]s",
				"name": "arm1",
				"type": "arm"
			},
			{
				"model": "%[1]s",
				"name": "arm2",
				"type": "arm"
			}
		],
	"jobs" : [
	{
		"name" : "arm1 job",
		"schedule" : "*/3 * * * * *",
		"resource" : "arm1",
		"method" : "DoCommand",
		"command" : {
			"command" : "test"
		}
	},
	{
		"name" : "arm2 job",
		"schedule" : "5s",
		"resource" : "arm2",
		"method" : "DoCommand",
		"command" : {
			"command" : "test"
		}
	}
	]
	} 
	`, model.String())
	defer func() {
		resource.Deregister(arm.API, model)
	}()

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(armConfig), logger, nil)
	test.That(t, err, test.ShouldBeNil)

	logger.Infow("here are jobs", "jobs", cfg.Jobs)
	ctx := context.Background()
	setupLocalRobot(t, ctx, cfg, logger)

	testutils.WaitForAssertionWithSleep(t, time.Second, 20, func(tb testing.TB) {
		tb.Helper()
		//test.That(tb, logs.FilterMessage("Job triggered").Len(),
		//test.ShouldBeGreaterThanOrEqualTo, 1)
		//test.That(tb, logs.FilterMessage("Job succeeded").Len(),
		//test.ShouldBeGreaterThanOrEqualTo, 1)

		test.That(tb, doCommandCount1, test.ShouldEqual, 5)
		test.That(tb, doCommandCount2, test.ShouldEqual, 3)
	})

	// Test OPID cancellation
	//options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	//err = r.StartWeb(ctx, options)
	//test.That(t, err, test.ShouldBeNil)

	//conn, err := rgrpc.Dial(ctx, addr, logger)
	//test.That(t, err, test.ShouldBeNil)
	//defer utils.UncheckedErrorFunc(conn.Close)
	//arm1, err := arm.NewClientFromConn(ctx, conn, "somerem", arm.Named("arm1"), logger)
	//test.That(t, err, test.ShouldBeNil)

	//foundOPID := false
	//stopAllErrCh := make(chan error, 1)
	//go func() {
	//<-channel
	//for _, opid := range r.OperationManager().All() {
	//if opid.Method == "/viam.component.arm.v1.ArmService/DoCommand" {
	//foundOPID = true
	//stopAllErrCh <- r.StopAll(ctx, nil)
	//}
	//}
	//}()
	//_, err = arm1.DoCommand(ctx, map[string]interface{}{})
	//s, isGRPCErr := status.FromError(err)
	//test.That(t, isGRPCErr, test.ShouldBeTrue)
	//test.That(t, s.Code(), test.ShouldEqual, codes.Canceled)

	//stopAllErr := <-stopAllErrCh
	//test.That(t, foundOPID, test.ShouldBeTrue)
	//test.That(t, stopAllErr, test.ShouldBeNil)
}
