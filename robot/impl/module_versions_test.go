package robotimpl

import (
	"path/filepath"
	"testing"
	"time"

	apppb "go.viam.com/api/app/v1"
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
		},
		Packages: pkgs,
	}
	applyModulePackageVersions(cfg)
	test.That(t, cfg.Modules[0].PackageVersion, test.ShouldEqual, "1.2.3")
	test.That(t, cfg.Modules[1].PackageVersion, test.ShouldBeEmpty)
	test.That(t, cfg.Modules[2].PackageVersion, test.ShouldBeEmpty)

	t.Run("module outside the packages directory is a local path reload", func(t *testing.T) {
		for _, p := range []string{
			"/home/dev/cam/bin/run",
			filepath.Join(config.DefaultPackagesDir()+config.LocalPackagesSuffix, "viam_reg-module.tar.gz", "bin", "run"),
		} {
			cfg.Modules[0].ExePath = p
			applyModulePackageVersions(cfg)
			test.That(t, cfg.Modules[0].PackageVersion, test.ShouldEqual, localReloadVersion)
		}
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
			RunningVersion: "1.0.0",
			RunningFor:     durationpb.New(time.Hour),
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
			infos := moduleVersionInfos(
				[]config.Module{registry("cam", "1.0.0")},
				[]modulestatus.Status{status("cam", state, "1.0.0")},
				now,
			)
			test.That(t, infos, test.ShouldHaveLength, 1)
			test.That(t, infos[0], test.ShouldResembleProto, &apppb.ModuleVersionInfo{ModuleId: "viam:cam"})
		}

		infos := moduleVersionInfos([]config.Module{registry("cam", "1.0.0")}, nil, now)
		test.That(t, infos, test.ShouldHaveLength, 1)
		test.That(t, infos[0], test.ShouldResembleProto, &apppb.ModuleVersionInfo{ModuleId: "viam:cam"})
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

	t.Run("local modules are excluded", func(t *testing.T) {
		infos := moduleVersionInfos(
			[]config.Module{
				{Name: "local", Type: config.ModuleTypeLocal, ExePath: "/bin/local"},
				{Name: "local-with-id", Type: config.ModuleTypeLocal, ModuleID: "viam:local-with-id"},
				registry("cam", "1.0.0"),
			},
			[]modulestatus.Status{
				status("local", modulestatus.ModuleStateReady, ""),
				status("local-with-id", modulestatus.ModuleStateReady, ""),
				status("cam", modulestatus.ModuleStateReady, "1.0.0"),
			},
			now,
		)
		test.That(t, infos, test.ShouldHaveLength, 1)
		test.That(t, infos[0].ModuleId, test.ShouldEqual, "viam:cam")
	})

	t.Run("reload builds report their reload version", func(t *testing.T) {
		for _, version := range []string{
			"reload-part-123-1700000000",
			"reload-source-part-123-1700000000",
			localReloadVersion,
		} {
			infos := moduleVersionInfos(
				[]config.Module{registry("cam", version)},
				[]modulestatus.Status{status("cam", modulestatus.ModuleStateReady, version)},
				now,
			)
			test.That(t, infos, test.ShouldHaveLength, 1)
			test.That(t, infos[0], test.ShouldResembleProto, &apppb.ModuleVersionInfo{
				ModuleId:       "viam:cam",
				RunningVersion: version,
				RunningFor:     durationpb.New(time.Hour),
			})
		}
	})
}
