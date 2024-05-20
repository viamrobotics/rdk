package module_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
)

func TestReconfigurationHappensConcurrently(t *testing.T) {
	ctx := context.Background()
	logger, _ := logging.NewObservedTestLogger(t)

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:     "TestModule",
				ExePath:  "testmodule/run.sh",
				LogLevel: "debug",
			},
		},
	}
	const resCount = 10
	for i := 1; i <= resCount; i++ {
		cfg.Components = append(cfg.Components,
			resource.Config{
				Name:  fmt.Sprintf("slow%d", i),
				Model: resource.NewModel("rdk", "test", "slow"),
				API:   generic.API,
			},
		)
	}

	start := time.Now()
	robot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer robot.Close(ctx)
	duration := time.Since(start)

	// We created a robot with 10 resources - each resource takes 1s to construct.
	// However, since we construct resources concurrently, it should take much less than
	// 10s. The threshold is somewhat arbitrary and can be adjusted upward if this test
	// starts to flake.
	const threshold = 3 * time.Second
	test.That(t, duration, test.ShouldBeLessThan, threshold)

	// This threshold should not be adjusted.
	const absThreshold = 10 * time.Second
	test.That(t, duration, test.ShouldBeLessThan, absThreshold)

	// Assert that all resources were added.
	for i := 1; i <= resCount; i++ {
		_, err = robot.ResourceByName(generic.Named(fmt.Sprintf("slow%d", i)))
		test.That(t, err, test.ShouldBeNil)
	}
}
