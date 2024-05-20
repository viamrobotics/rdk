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
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
)

func setupTestRobotWithModules(
	t *testing.T,
	ctx context.Context,
	logger logging.Logger,
) (*config.Config, robot.LocalRobot) {
	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "TestModule",
				ExePath: "testmodule/run.sh",
			},
		},
	}
	rob, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, rob.Close(ctx), test.ShouldBeNil)
	})
	return cfg, rob
}

func TestConcurrentReconfiguration(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("no dependencies", func(t *testing.T) {
		cfg, rob := setupTestRobotWithModules(t, ctx, logger)

		// Add resources to the config
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
		// We configure the robot with 10 resources where each resource takes 1s to
		// construct. Since we construct resources concurrently, it should take much less
		// than 10s.
		start := time.Now()
		rob.Reconfigure(ctx, cfg)
		duration := time.Since(start)

		// This threshold is somewhat arbitrary and can be adjusted upward if this test
		// starts to flake.
		const threshold = 3 * time.Second
		test.That(t, duration, test.ShouldBeLessThan, threshold)

		// This threshold is absolute and should not be adjusted.
		const absThreshold = 10 * time.Second
		test.That(t, duration, test.ShouldBeLessThan, absThreshold)

		// Assert that all resources were added.
		var err error
		for i := 1; i <= resCount; i++ {
			_, err = rob.ResourceByName(generic.Named(fmt.Sprintf("slow%d", i)))
			test.That(t, err, test.ShouldBeNil)
		}

		// Rename resources in config
		cfg.Components = nil
		for i := 1; i <= resCount; i++ {
			// rename components
			cfg.Components = append(cfg.Components,
				resource.Config{
					Name:  fmt.Sprintf("slow-%d", i),
					Model: resource.NewModel("rdk", "test", "slow"),
					API:   generic.API,
				},
			)
		}

		// We renamed each resource to trigger a reconfiguration - this should happen
		// concurrently and take much less than 10s.
		start = time.Now()
		rob.Reconfigure(context.Background(), cfg)
		duration = time.Since(start)

		test.That(t, duration, test.ShouldBeLessThan, threshold)
		test.That(t, duration, test.ShouldBeLessThan, absThreshold)

		// Assert that all resources were reconfigured.
		for i := 1; i <= resCount; i++ {
			_, err = rob.ResourceByName(generic.Named(fmt.Sprintf("slow%d", i)))
			test.That(t, err, test.ShouldNotBeNil)
			_, err = rob.ResourceByName(generic.Named(fmt.Sprintf("slow-%d", i)))
			test.That(t, err, test.ShouldBeNil)
		}
	})

	t.Run("with dependencies", func(t *testing.T) {
		cfg, rob := setupTestRobotWithModules(t, ctx, logger)
		const resCount = 10
		for i := 1; i <= resCount; i++ {
			if i%2 == 1 {
				cfg.Components = append(cfg.Components,
					resource.Config{
						Name:  fmt.Sprintf("slow%d", i),
						Model: resource.NewModel("rdk", "test", "slow"),
						API:   generic.API,
					},
				)
			} else {
				cfg.Components = append(cfg.Components,
					resource.Config{
						Name:      fmt.Sprintf("slow%d", i),
						Model:     resource.NewModel("rdk", "test", "slow"),
						API:       generic.API,
						DependsOn: []string{fmt.Sprintf("slow%d", i-1)},
					},
				)
			}
		}

		// We configure the robot with 10 resources where each resource takes 1s to
		// construct. This resource config has 2 levels of dependencies and resources are
		// constructed concurrently within each level. Therefore it should take more than
		// 2s to reconfigure but much less than 10s.
		start := time.Now()
		rob.Reconfigure(context.Background(), cfg)
		duration := time.Since(start)

		test.That(t, duration, test.ShouldBeGreaterThanOrEqualTo, 2*time.Second)
		// This threshold is somewhat arbitrary and can be adjusted upward if this test
		// starts to flake.
		test.That(t, duration, test.ShouldBeLessThan, 5*time.Second)
		// This threshold is absolute and should not be adjusted.
		test.That(t, duration, test.ShouldBeLessThan, 10*time.Second)

		// Assert that all resources were added.
		var err error
		for i := 1; i <= resCount; i++ {
			_, err = rob.ResourceByName(generic.Named(fmt.Sprintf("slow%d", i)))
			test.That(t, err, test.ShouldBeNil)
		}
	})
}
