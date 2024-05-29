package module_test

import (
	"context"
	"fmt"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/utils"
)

func setupTestRobotWithModules(
	t testing.TB,
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

const resCount = 10
const configDuration = "1ms"

func BenchmarkConcurrentReconfiguration(b *testing.B) {
	ctx := context.Background()
	logger := logging.NewTestLogger(b)

	reportMetrics := func(b *testing.B) {
		// How many resources are reconfigured per millisecond. Since the number of
		// resources and configuration for each resource is constant, this metric should
		// generally as there are fewer dependencies between resources.
		b.ReportMetric(float64(b.Elapsed().Milliseconds())/float64(b.N)/float64(resCount), "ms/resource")
	}

	type testcase struct {
		name         string
		dependencies [][]string
	}
	for _, tc := range []testcase{
		{"serial dependencies", func() (deps [][]string) {
			// Update config to include N resources such that:
			//
			// * resource 0 has no dependencies
			// * for all resources N>0, resource N depends on resource N-1
			//
			// This dependency structure should force each resource to be constructed
			// serially. If each resource takes duration T to configure, then
			// total configuration time per resource be approximately T.
			for i := 0; i < resCount; i++ {
				var dependsOn []string
				if i > 0 {
					dependsOn = append(dependsOn, fmt.Sprintf("slow%d", i-1))
				}
				deps = append(deps, dependsOn)
			}
			return
		}()},
		{"no dependencies", func() (deps [][]string) {
			// Update config to include N resources that do no have any dependencies.
			//
			// This dependency structure should allow each resource to be constructed
			// concurrently. If each resource takes duration T to configure, then
			// total configuration time per resource be approximately T/N.
			for i := 0; i < resCount; i++ {
				deps = append(deps, nil)
			}
			return
		}()},
		{"some dependencies", func() (deps [][]string) {
			// Update config to include N resources such that for approximately 1/3 of
			// the resources, resource N depends on resource N-1.
			//
			// This dependency structure should allow some resources to be constructed
			// concurrently. If each resource takes duration T to configure, then
			// total configuration time per resource be between T/N and T, approximately.
			for i := 0; i < resCount; i++ {
				var dependsOn []string
				if i%3 == 1 {
					dependsOn = append(dependsOn, fmt.Sprintf("slow%d", i-1))
				}
				deps = append(deps, dependsOn)
			}
			return
		}()},
	} {
		b.Run(tc.name, func(b *testing.B) {
			cfg, rob := setupTestRobotWithModules(b, ctx, logger)

			b.StopTimer()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				// Create config with parametrized dependencies.
				for i := 0; i < resCount; i++ {
					cfg.Components = append(cfg.Components,
						resource.Config{
							Name:       fmt.Sprintf("slow%d", i),
							Model:      resource.NewModel("rdk", "test", "slow"),
							API:        generic.API,
							Attributes: utils.AttributeMap{"config_duration": configDuration},
							DependsOn:  tc.dependencies[i],
						},
					)
				}

				// Reconfigure robot and benchmark.
				b.StartTimer()
				rob.Reconfigure(ctx, cfg)
				b.StopTimer()

				// Assert that all resources were added.
				var err error
				for i := 0; i < resCount; i++ {
					_, err = rob.ResourceByName(generic.Named(fmt.Sprintf("slow%d", i)))
					test.That(b, err, test.ShouldBeNil)
				}
				// Remove all resources.
				cfg.Components = nil
				rob.Reconfigure(ctx, cfg)
				test.That(b, rob.ResourceNames(), test.ShouldBeEmpty)
			}
			reportMetrics(b)
		})
	}
}
