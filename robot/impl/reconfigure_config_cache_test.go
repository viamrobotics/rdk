package robotimpl

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/packages"
	"go.viam.com/rdk/testutils/inject"
)

// syncFailingPackageManager fails Sync on demand so tests can force reconfigure to abort
// at the package sync step.
type syncFailingPackageManager struct {
	packages.ManagerSyncer
	fail atomic.Bool
}

func (m *syncFailingPackageManager) Sync(ctx context.Context, pkgs []config.PackageConfig, modules []config.Module) error {
	if m.fail.Load() {
		return errors.New("injected package sync failure")
	}
	return m.ManagerSyncer.Sync(ctx, pkgs, modules)
}

// registerToggleMaintenanceSensor registers a sensor model whose "allowed" reading tracks
// the returned flag.
func registerToggleMaintenanceSensor(t *testing.T) (resource.Model, *atomic.Bool) {
	t.Helper()
	allowed := &atomic.Bool{}
	allowed.Store(true)
	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	resource.RegisterComponent(
		sensor.API,
		model,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sensor.Sensor, error) {
			s := &inject.Sensor{}
			s.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{"allowed": allowed.Load()}, nil
			}
			return s, nil
		}},
	)
	t.Cleanup(func() { resource.Deregister(sensor.API, model) })
	return model, allowed
}

func tunnelNetworkConfig(port int) config.NetworkConfig {
	return config.NetworkConfig{
		NetworkConfigData: config.NetworkConfigData{
			TrafficTunnelEndpoints: []config.TrafficTunnelEndpoint{{Port: port, ConnectionTimeout: time.Second}},
		},
	}
}

func tracingEnabledConfig() config.TracingConfig {
	return config.TracingConfig{Enabled: true, Disk: true}
}

// TestConfigCacheMaintenanceBlocked covers what the robot reports and later applies when
// a config push is rejected by the maintenance gate.
func TestConfigCacheMaintenanceBlocked(t *testing.T) {
	ctx := context.Background()
	model, allowed := registerToggleMaintenanceSensor(t)
	maintenanceCfg := &config.MaintenanceConfig{SensorName: "rdk:component:sensor/maint", MaintenanceAllowedKey: "allowed"}
	maintSensor := resource.Config{Name: "maint", API: sensor.API, Model: model}
	otherSensor := resource.Config{Name: "other", API: sensor.API, Model: model}

	t.Run("network settings applied by the entrypoint are reported while blocked", func(t *testing.T) {
		allowed.Store(true)
		logger := logging.NewTestLogger(t)
		r := setupLocalRobot(t, ctx, &config.Config{
			MaintenanceConfig: maintenanceCfg,
			Components:        []resource.Config{maintSensor},
			Network:           tunnelNetworkConfig(9090),
		}, logger)

		allowed.Store(false)
		r.Reconfigure(ctx, &config.Config{
			MaintenanceConfig: maintenanceCfg,
			Components:        []resource.Config{maintSensor, otherSensor},
			Network:           tunnelNetworkConfig(9191),
		})

		_, err := r.ResourceByName(sensor.Named("other"))
		test.That(t, err, test.ShouldBeError, nodeNotFoundError("other", sensor.API))

		// The entrypoint restarts the web service with the new network config regardless of
		// the maintenance gate, so the robot must not keep reporting the old one.
		tunnels, err := r.ListTunnels(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, tunnels, test.ShouldResemble, tunnelNetworkConfig(9191).TrafficTunnelEndpoints)
	})

	t.Run("blocked resource change is still applied once unblocked", func(t *testing.T) {
		allowed.Store(true)
		logger := logging.NewTestLogger(t)
		r := setupLocalRobot(t, ctx, &config.Config{
			Revision:          "rev1",
			MaintenanceConfig: maintenanceCfg,
			Components:        []resource.Config{maintSensor},
		}, logger)
		newCfg := func() *config.Config {
			return &config.Config{
				Revision:          "rev2",
				MaintenanceConfig: maintenanceCfg,
				Components:        []resource.Config{maintSensor, otherSensor},
			}
		}

		allowed.Store(false)
		r.Reconfigure(ctx, newCfg())
		_, err := r.ResourceByName(sensor.Named("other"))
		test.That(t, err, test.ShouldBeError, nodeNotFoundError("other", sensor.API))
		mStatus, err := r.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, "rev1")

		allowed.Store(true)
		r.Reconfigure(ctx, newCfg())
		_, err = r.ResourceByName(sensor.Named("other"))
		test.That(t, err, test.ShouldBeNil)
		mStatus, err = r.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, "rev2")
	})

	t.Run("blocked tracing change is still applied once unblocked", func(t *testing.T) {
		// TODO: a tracing (or jobs) change pushed while reconfigure is blocked is stored as
		// the most recent config without being applied, so the identical push after the gate
		// opens diffs as a no-op and the change never takes effect.
		t.Skip("blocked tracing changes are lost once unblocked")
		allowed.Store(true)
		// localRobot.Close shuts down the global trace objects.
		trace.SetProvider(t.Context())
		logger := logging.NewTestLogger(t)
		r := setupLocalRobot(t, ctx, &config.Config{
			MaintenanceConfig: maintenanceCfg,
			Components:        []resource.Config{maintSensor},
		}, logger).(*localRobot)
		newCfg := func() *config.Config {
			return &config.Config{
				MaintenanceConfig: maintenanceCfg,
				Components:        []resource.Config{maintSensor},
				Tracing:           tracingEnabledConfig(),
			}
		}

		allowed.Store(false)
		r.Reconfigure(ctx, newCfg())
		test.That(t, r.traceClients.Load(), test.ShouldBeNil)

		// The blocked push must not be remembered as applied, or this identical push would
		// diff as a no-op and tracing would never turn on.
		allowed.Store(true)
		r.Reconfigure(ctx, newCfg())
		test.That(t, r.traceClients.Load(), test.ShouldNotBeNil)
		test.That(t, *r.traceClients.Load(), test.ShouldHaveLength, 1)
	})

	t.Run("blocked maintenance config change still gates module restarts", func(t *testing.T) {
		allowed.Store(true)
		logger := logging.NewTestLogger(t)
		r := setupLocalRobot(t, ctx, &config.Config{
			Components: []resource.Config{maintSensor},
		}, logger).(*localRobot)
		test.That(t, r.RestartAllowed(), test.ShouldBeTrue)

		// The first push that introduces a blocking maintenance config is itself blocked.
		allowed.Store(false)
		r.Reconfigure(ctx, &config.Config{
			MaintenanceConfig: maintenanceCfg,
			Components:        []resource.Config{maintSensor, otherSensor},
		})
		test.That(t, r.RestartAllowed(), test.ShouldBeFalse)
		mStatus, err := r.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.MaintenanceStatus, test.ShouldNotBeNil)
		test.That(t, mStatus.MaintenanceStatus.Allowed, test.ShouldBeFalse)
	})
}

// TestConfigCacheAbortedReconfigure covers reconfigures that abort after the maintenance
// gate, where some of the new config's side effects have already been applied.
func TestConfigCacheAbortedReconfigure(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T, cfg *config.Config) (*localRobot, *syncFailingPackageManager) {
		t.Helper()
		// localRobot.Close shuts down the global trace objects.
		trace.SetProvider(t.Context())
		r := setupLocalRobot(t, ctx, cfg, logging.NewTestLogger(t)).(*localRobot)
		pm := &syncFailingPackageManager{ManagerSyncer: r.packageManager}
		r.reconfigurationLock.Lock()
		r.packageManager = pm
		r.reconfigurationLock.Unlock()
		return r, pm
	}

	t.Run("tracing is not rebuilt on every retry of an aborted reconfigure", func(t *testing.T) {
		r, pm := setup(t, &config.Config{})
		pm.fail.Store(true)

		r.Reconfigure(ctx, &config.Config{Tracing: tracingEnabledConfig()})
		firstClients := r.traceClients.Load()
		test.That(t, firstClients, test.ShouldNotBeNil)

		r.Reconfigure(ctx, &config.Config{Tracing: tracingEnabledConfig()})
		test.That(t, r.traceClients.Load() == firstClients, test.ShouldBeTrue)
	})

	t.Run("tracing enabled by an aborted reconfigure can be disabled again", func(t *testing.T) {
		r, pm := setup(t, &config.Config{})
		pm.fail.Store(true)

		r.Reconfigure(ctx, &config.Config{Tracing: tracingEnabledConfig()})
		test.That(t, r.traceClients.Load(), test.ShouldNotBeNil)

		// Reverting to the last fully applied config must undo the tracing change the
		// aborted reconfigure already made.
		pm.fail.Store(false)
		r.Reconfigure(ctx, &config.Config{})
		test.That(t, r.traceClients.Load(), test.ShouldBeNil)
	})

	t.Run("tracing disabled by an aborted reconfigure can be enabled again", func(t *testing.T) {
		r, pm := setup(t, &config.Config{Tracing: tracingEnabledConfig()})
		test.That(t, r.traceClients.Load(), test.ShouldNotBeNil)
		pm.fail.Store(true)

		r.Reconfigure(ctx, &config.Config{})
		test.That(t, r.traceClients.Load(), test.ShouldBeNil)

		pm.fail.Store(false)
		r.Reconfigure(ctx, &config.Config{Tracing: tracingEnabledConfig()})
		test.That(t, r.traceClients.Load(), test.ShouldNotBeNil)
	})

	t.Run("network settings applied by the entrypoint are reported after an abort", func(t *testing.T) {
		r, pm := setup(t, &config.Config{Network: tunnelNetworkConfig(9090)})
		pm.fail.Store(true)

		r.Reconfigure(ctx, &config.Config{Network: tunnelNetworkConfig(9191)})

		tunnels, err := r.ListTunnels(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, tunnels, test.ShouldResemble, tunnelNetworkConfig(9191).TrafficTunnelEndpoints)
	})

	t.Run("resources from an aborted reconfigure are applied on the next successful one", func(t *testing.T) {
		model, _ := registerToggleMaintenanceSensor(t)
		newCfg := func() *config.Config {
			return &config.Config{
				Revision:   "rev2",
				Components: []resource.Config{{Name: "other", API: sensor.API, Model: model}},
			}
		}
		r, pm := setup(t, &config.Config{Revision: "rev1"})
		pm.fail.Store(true)

		r.Reconfigure(ctx, newCfg())
		_, err := r.ResourceByName(sensor.Named("other"))
		test.That(t, err, test.ShouldBeError, nodeNotFoundError("other", sensor.API))
		mStatus, err := r.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, "rev1")

		pm.fail.Store(false)
		r.Reconfigure(ctx, newCfg())
		_, err = r.ResourceByName(sensor.Named("other"))
		test.That(t, err, test.ShouldBeNil)
		mStatus, err = r.MachineStatus(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mStatus.Config.Revision, test.ShouldEqual, "rev2")
	})
}

// TestConfigCacheStartupAbort covers the very first reconfigure aborting, when there is
// no previously applied config to fall back to.
func TestConfigCacheStartupAbort(t *testing.T) {
	ctx := context.Background()
	trace.SetProvider(t.Context())

	// A local tarball module whose tarball does not exist fails local package sync.
	missingTarball := config.Module{
		Name:    "missing",
		Type:    config.ModuleTypeLocal,
		ExePath: filepath.Join(t.TempDir(), "missing.tar.gz"),
	}
	cfg := &config.Config{
		Modules: []config.Module{missingTarball},
		Network: tunnelNetworkConfig(9090),
		Tracing: tracingEnabledConfig(),
	}
	r := setupLocalRobot(t, ctx, cfg, logging.NewTestLogger(t)).(*localRobot)
	test.That(t, r.Config().Modules, test.ShouldBeEmpty)

	t.Run("non-resource config is reported", func(t *testing.T) {
		tunnels, err := r.ListTunnels(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, tunnels, test.ShouldResemble, cfg.Network.TrafficTunnelEndpoints)
	})

	t.Run("tracing is not rebuilt when the same config is retried", func(t *testing.T) {
		firstClients := r.traceClients.Load()
		test.That(t, firstClients, test.ShouldNotBeNil)

		r.Reconfigure(ctx, &config.Config{
			Modules: []config.Module{missingTarball},
			Network: tunnelNetworkConfig(9090),
			Tracing: tracingEnabledConfig(),
		})
		test.That(t, r.traceClients.Load() == firstClients, test.ShouldBeTrue)
	})
}
