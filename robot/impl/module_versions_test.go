package robotimpl

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	apppb "go.viam.com/api/app/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/config"
	modulestatus "go.viam.com/rdk/module/status"
)

func TestApplyModulePackageVersions(t *testing.T) {
	pkgs := []config.PackageConfig{
		{Name: "m_reg", Package: "org/reg", Version: "1.2.3", Type: config.PackageTypeModule},
		{Name: "m_local", Package: "org/local", Version: "9.9.9", Type: config.PackageTypeModule},
		{Name: "m_no-pkg", Package: "org/model", Version: "4.0.0", Type: config.PackageTypeMlModel},
	}
	exePath := func(i int) string {
		return filepath.Join(pkgs[i].LocalDataDirectory(config.DefaultPackagesDir()), "bin", "run")
	}
	cfg := &config.Config{
		Modules: []config.Module{
			{Name: "reg", Type: config.ModuleTypeRegistry, ModuleID: "viam:reg", ExePath: exePath(0)},
			{Name: "local", Type: config.ModuleTypeLocal, ExePath: exePath(1)},
			{Name: "no-pkg", Type: config.ModuleTypeRegistry, ModuleID: "viam:no-pkg", ExePath: exePath(2)},
			{Name: "reloaded", Type: config.ModuleTypeLocal, ModuleID: "viam:reloaded", ExePath: "/home/dev/cam/bin/run"},
		},
		Packages: pkgs,
	}
	applyModulePackageVersions(cfg)
	test.That(t, cfg.Modules[0].PackageVersion, test.ShouldEqual, "1.2.3")
	test.That(t, cfg.Modules[1].PackageVersion, test.ShouldEqual, localVersion)
	test.That(t, cfg.Modules[2].PackageVersion, test.ShouldBeEmpty)
	test.That(t, cfg.Modules[3].PackageVersion, test.ShouldEqual, localVersion)

	t.Run("registry module outside any package directory gets no version", func(t *testing.T) {
		cfg.Modules[0].ExePath = "/home/dev/cam/bin/run"
		applyModulePackageVersions(cfg)
		test.That(t, cfg.Modules[0].PackageVersion, test.ShouldBeEmpty)
	})

	t.Run("package directory must contain the ExePath, not just prefix it", func(t *testing.T) {
		other := config.PackageConfig{Name: "m_reg", Package: "org/reg", Version: "1.2.30", Type: config.PackageTypeModule}
		cfg.Modules[0].ExePath = filepath.Join(other.LocalDataDirectory(config.DefaultPackagesDir()), "bin", "run")
		applyModulePackageVersions(cfg)
		test.That(t, cfg.Modules[0].PackageVersion, test.ShouldBeEmpty)
	})
}

func TestModuleVersionInfos(t *testing.T) {
	now := time.Now()
	readyAt := now.Add(-time.Hour)

	registry := func(name, version string) config.Module {
		return config.Module{Name: name, Type: config.ModuleTypeRegistry, ModuleID: "viam:" + name, PackageVersion: version}
	}
	status := func(name string, state modulestatus.State, version string) modulestatus.Status {
		return modulestatus.Status{Name: name, State: state, LastUpdated: readyAt, Version: version}
	}

	t.Run("ready module reports running version and duration", func(t *testing.T) {
		infos := moduleVersionInfos(
			[]config.Module{registry("cam", "1.0.0")},
			[]modulestatus.Status{status("cam", modulestatus.ModuleStateReady, "1.0.0")},
			now,
		)
		test.That(t, infos, test.ShouldHaveLength, 1)
		test.That(t, infos[0], test.ShouldResembleProto, &apppb.ModuleVersionInfo{
			ModuleId:       "viam:cam",
			ModuleName:     "cam",
			RunningVersion: "1.0.0",
			RunningFor:     durationpb.New(time.Hour),
			State:          robotpb.ModuleStatus_STATE_READY,
		})
	})

	t.Run("module that is not running reports empty version and unset duration", func(t *testing.T) {
		for _, state := range []modulestatus.State{
			modulestatus.ModuleStateUnknown,
			modulestatus.ModuleStatePending,
			modulestatus.ModuleStateStarting,
			modulestatus.ModuleStateUnhealthy,
			modulestatus.ModuleStateClosing,
		} {
			s := status("cam", state, "1.0.0")
			infos := moduleVersionInfos([]config.Module{registry("cam", "1.0.0")}, []modulestatus.Status{s}, now)
			test.That(t, infos, test.ShouldHaveLength, 1)
			test.That(t, infos[0], test.ShouldResembleProto, &apppb.ModuleVersionInfo{
				ModuleId:   "viam:cam",
				ModuleName: "cam",
				State:      s.ToProto().State,
			})
		}

		infos := moduleVersionInfos([]config.Module{registry("cam", "1.0.0")}, nil, now)
		test.That(t, infos, test.ShouldHaveLength, 1)
		test.That(t, infos[0], test.ShouldResembleProto, &apppb.ModuleVersionInfo{ModuleId: "viam:cam", ModuleName: "cam"})
	})

	t.Run("module status is reported", func(t *testing.T) {
		s := status("cam", modulestatus.ModuleStateUnhealthy, "1.0.0")
		s.Error = errors.New("exit status 2")
		s.ConsecutiveFailures = 3
		s.ReadyCount = 4
		infos := moduleVersionInfos([]config.Module{registry("cam", "1.0.0")}, []modulestatus.Status{s}, now)
		test.That(t, infos, test.ShouldHaveLength, 1)
		test.That(t, infos[0], test.ShouldResembleProto, &apppb.ModuleVersionInfo{
			ModuleId:            "viam:cam",
			ModuleName:          "cam",
			State:               robotpb.ModuleStatus_STATE_UNHEALTHY,
			Error:               "exit status 2",
			ConsecutiveFailures: 3,
			ReadyCount:          4,
		})
	})

	t.Run("update in flight reports the version that is still running", func(t *testing.T) {
		infos := moduleVersionInfos(
			[]config.Module{registry("cam", "2.0.0")},
			[]modulestatus.Status{status("cam", modulestatus.ModuleStateReady, "1.0.0")},
			now,
		)
		test.That(t, infos, test.ShouldHaveLength, 1)
		test.That(t, infos[0].RunningVersion, test.ShouldEqual, "1.0.0")
	})

	t.Run("local modules and local path reloads are reported", func(t *testing.T) {
		infos := moduleVersionInfos(
			[]config.Module{
				{Name: "local", Type: config.ModuleTypeLocal, ExePath: "/bin/local", PackageVersion: localVersion},
				{Name: "reloaded", Type: config.ModuleTypeLocal, ModuleID: "viam:reloaded", PackageVersion: localVersion},
				registry("cam", "1.0.0"),
			},
			[]modulestatus.Status{
				status("local", modulestatus.ModuleStateReady, localVersion),
				status("reloaded", modulestatus.ModuleStateReady, localVersion),
				status("cam", modulestatus.ModuleStateReady, "1.0.0"),
			},
			now,
		)
		test.That(t, infos, test.ShouldHaveLength, 3)
		test.That(t, infos[0], test.ShouldResembleProto, &apppb.ModuleVersionInfo{
			ModuleName:     "local",
			RunningVersion: localVersion,
			RunningFor:     durationpb.New(time.Hour),
			State:          robotpb.ModuleStatus_STATE_READY,
		})
		test.That(t, infos[1].ModuleId, test.ShouldEqual, "viam:reloaded")
		test.That(t, infos[1].RunningVersion, test.ShouldEqual, localVersion)
		test.That(t, infos[2].ModuleId, test.ShouldEqual, "viam:cam")
	})

	t.Run("cloud reload builds report their reload version", func(t *testing.T) {
		for _, version := range []string{
			"reload-part-123-1700000000",
			"reload-source-part-123-1700000000",
		} {
			infos := moduleVersionInfos(
				[]config.Module{registry("cam", version)},
				[]modulestatus.Status{status("cam", modulestatus.ModuleStateReady, version)},
				now,
			)
			test.That(t, infos, test.ShouldHaveLength, 1)
			test.That(t, infos[0], test.ShouldResembleProto, &apppb.ModuleVersionInfo{
				ModuleId:       "viam:cam",
				ModuleName:     "cam",
				RunningVersion: version,
				RunningFor:     durationpb.New(time.Hour),
				State:          robotpb.ModuleStatus_STATE_READY,
			})
		}
	})
}
