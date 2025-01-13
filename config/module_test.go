package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

// testChdir is a helper that cleans up an os.Chdir.
func testChdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	test.That(t, err, test.ShouldBeNil)
	err = os.Chdir(dir)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { os.Chdir(wd) })
}

func TestInternalMeta(t *testing.T) {
	tmp := t.TempDir()
	testChdir(t, tmp)
	testWriteJSON(t, "meta.json", JSONManifest{Entrypoint: "entry"})
	packagesDir := filepath.Join(tmp, "packages")
	t.Run("local-tarball", func(t *testing.T) {
		mod := Module{
			Type:    ModuleTypeLocal,
			ExePath: filepath.Join(tmp, "whatever.tar.gz"),
		}
		exePath, err := mod.EvaluateExePath(packagesDir)
		test.That(t, err, test.ShouldBeNil)
		exeDir, err := mod.exeDir(packagesDir)
		test.That(t, err, test.ShouldBeNil)
		// "entry" is from meta.json.
		test.That(t, exePath, test.ShouldEqual, filepath.Join(exeDir, "entry"))
	})

	t.Run("non-tarball", func(t *testing.T) {
		mod := Module{
			Type:    ModuleTypeLocal,
			ExePath: filepath.Join(tmp, "whatever"),
		}
		exePath, err := mod.EvaluateExePath(packagesDir)
		test.That(t, err, test.ShouldBeNil)
		// "whatever" is from config.Module object.
		test.That(t, exePath, test.ShouldEqual, filepath.Join(tmp, "whatever"))
	})
}

func TestSyntheticModule(t *testing.T) {
	tmp := t.TempDir()
	modNeedsSynthetic := Module{
		Type:    ModuleTypeLocal,
		ExePath: filepath.Join(tmp, "whatever.tgz"),
	}
	modNotTar := Module{
		Type:    ModuleTypeLocal,
		ExePath: "/home/user/whatever.sh",
	}
	modNotLocal := Module{
		Type: ModuleTypeRegistry,
	}

	t.Run("NeedsSyntheticPackage", func(t *testing.T) {
		test.That(t, modNeedsSynthetic.NeedsSyntheticPackage(), test.ShouldBeTrue)
		test.That(t, modNotTar.NeedsSyntheticPackage(), test.ShouldBeFalse)
		test.That(t, modNotLocal.NeedsSyntheticPackage(), test.ShouldBeFalse)
	})

	t.Run("SyntheticPackage", func(t *testing.T) {
		_, err := modNeedsSynthetic.SyntheticPackage()
		test.That(t, err, test.ShouldBeNil)
		_, err = modNotLocal.SyntheticPackage()
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("syntheticPackageExeDir", func(t *testing.T) {
		dir, err := modNeedsSynthetic.exeDir(tmp)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dir, test.ShouldEqual, filepath.Join(tmp, "data/module/synthetic--"))
	})

	t.Run("EvaluateExePath", func(t *testing.T) {
		meta := JSONManifest{
			Entrypoint: "entry",
		}
		testWriteJSON(t, filepath.Join(tmp, "meta.json"), &meta)

		// local tarball case
		syntheticPath, err := modNeedsSynthetic.EvaluateExePath(tmp)
		test.That(t, err, test.ShouldBeNil)
		exeDir, err := modNeedsSynthetic.exeDir(tmp)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, syntheticPath, test.ShouldEqual, filepath.Join(exeDir, meta.Entrypoint))

		// vanilla case
		notTarPath, err := modNotTar.EvaluateExePath(tmp)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, notTarPath, test.ShouldEqual, modNotTar.ExePath)
	})
}

func TestFirstRun(t *testing.T) {
	m := Module{Type: ModuleTypeRegistry}

	tmp := t.TempDir()
	exePath := filepath.Join(tmp, "whatever.sh")
	m.ExePath = exePath
	metaJSONFilepath := filepath.Join(tmp, "meta.json")

	ctx := context.Background()
	localPackagesDir := ""
	dataDir := ""
	env := map[string]string{"VIAM_MODULE_ROOT": tmp}
	logger, observedLogs := logging.NewObservedTestLogger(t)

	t.Run("MetaFileNotFound", func(t *testing.T) {
		err := m.FirstRun(ctx, localPackagesDir, dataDir, env, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, observedLogs.FilterMessage("meta.json not found, skipping first run").Len(), test.ShouldEqual, 1)
	})

	t.Run("MetaFileInvalid", func(t *testing.T) {
		metaJSONFile, err := os.Create(metaJSONFilepath)
		test.That(t, err, test.ShouldBeNil)
		defer metaJSONFile.Close()

		err = m.FirstRun(ctx, localPackagesDir, dataDir, env, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, observedLogs.FilterMessage("failed to parse meta.json, skipping first run").Len(), test.ShouldEqual, 1)
	})

	t.Run("NoFirstRunScript", func(t *testing.T) {
		testWriteJSON(t, metaJSONFilepath, JSONManifest{})

		err := m.FirstRun(ctx, localPackagesDir, dataDir, env, logger)
		t.Log(err)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, observedLogs.FilterMessage("no first run script specified, skipping first run").Len(), test.ShouldEqual, 1)
	})

	t.Run("InvalidFirstRunPath", func(t *testing.T) {
		testWriteJSON(t, metaJSONFilepath, JSONManifest{FirstRun: "../firstrun.sh"})

		err := m.FirstRun(ctx, localPackagesDir, dataDir, env, logger)
		t.Log(err)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, observedLogs.FilterMessage("failed to build path to first run script, skipping first run").Len(), test.ShouldEqual, 1)
	})
}

func TestGetJSONManifest(t *testing.T) {
	validJSONManifest := JSONManifest{Entrypoint: "entry"}

	t.Run("RegistryModule", func(t *testing.T) {
		tmp := t.TempDir()

		topLevelDir := tmp
		topLevelMetaJSONFilepath := filepath.Join(topLevelDir, "meta.json")
		unpackedModDir := filepath.Join(tmp, "unpacked-mod-dir")
		unpackedModMetaJSONFilepath := filepath.Join(unpackedModDir, "meta.json")
		env := make(map[string]string, 1)
		modRegistry := Module{Type: ModuleTypeRegistry}

		err := os.Mkdir(unpackedModDir, 0o700)
		test.That(t, err, test.ShouldBeNil)

		// meta.json not found; only unpacked module directory searched
		meta, moduleWorkingDirectory, err := modRegistry.getJSONManifest(unpackedModDir, env)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, moduleWorkingDirectory, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "registry module")
		test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldBeTrue)
		test.That(t, err.Error(), test.ShouldContainSubstring, unpackedModMetaJSONFilepath)
		test.That(t, err.Error(), test.ShouldNotContainSubstring, topLevelMetaJSONFilepath)

		// meta.json not found; top level module directory and unpacked module directories searched
		env["VIAM_MODULE_ROOT"] = topLevelDir

		meta, moduleWorkingDirectory, err = modRegistry.getJSONManifest(unpackedModDir, env)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, moduleWorkingDirectory, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "registry module")
		test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldBeTrue)
		test.That(t, err.Error(), test.ShouldContainSubstring, unpackedModMetaJSONFilepath)
		test.That(t, err.Error(), test.ShouldContainSubstring, topLevelMetaJSONFilepath)

		// meta.json found in unpacked modular directory; parsing fails
		unpackedModMetaJSONFile, err := os.Create(unpackedModMetaJSONFilepath)
		test.That(t, err, test.ShouldBeNil)
		defer unpackedModMetaJSONFile.Close()

		meta, moduleWorkingDirectory, err = modRegistry.getJSONManifest(unpackedModDir, env)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, moduleWorkingDirectory, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "registry module")
		test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldBeFalse)

		// meta.json found in unpacked modular directory; parsing succeeds
		testWriteJSON(t, unpackedModMetaJSONFilepath, validJSONManifest)

		meta, moduleWorkingDirectory, err = modRegistry.getJSONManifest(unpackedModDir, env)
		test.That(t, *meta, test.ShouldResemble, validJSONManifest)
		test.That(t, moduleWorkingDirectory, test.ShouldEqual, unpackedModDir)
		test.That(t, err, test.ShouldBeNil)

		// meta.json found in top level modular directory; parsing fails
		topLevelMetaJSONFile, err := os.Create(topLevelMetaJSONFilepath)
		test.That(t, err, test.ShouldBeNil)
		defer topLevelMetaJSONFile.Close()

		meta, moduleWorkingDirectory, err = modRegistry.getJSONManifest(unpackedModDir, env)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, moduleWorkingDirectory, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "registry module")
		test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldBeFalse)

		// meta.json found in top level modular directory; parsing succeeds
		testWriteJSON(t, topLevelMetaJSONFilepath, validJSONManifest)

		meta, moduleWorkingDirectory, err = modRegistry.getJSONManifest(unpackedModDir, env)
		test.That(t, *meta, test.ShouldResemble, validJSONManifest)
		test.That(t, moduleWorkingDirectory, test.ShouldEqual, topLevelDir)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("LocalTarball", func(t *testing.T) {
		tmp := t.TempDir()

		exePath := filepath.Join(tmp, "module.tgz")
		exeDir := filepath.Dir(exePath)
		exeMetaJSONFilepath := filepath.Join(exeDir, "meta.json")
		unpackedModDir := filepath.Join(tmp, "unpacked-mod-dir")
		unpackedModMetaJSONFilepath := filepath.Join(unpackedModDir, "meta.json")
		env := map[string]string{}
		modLocalTar := Module{Type: ModuleTypeLocal, ExePath: exePath}

		err := os.Mkdir(unpackedModDir, 0o700)
		test.That(t, err, test.ShouldBeNil)

		// meta.json not found; unpacked module and executable directories searched
		meta, moduleWorkingDirectory, err := modLocalTar.getJSONManifest(unpackedModDir, env)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, moduleWorkingDirectory, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "local tarball")
		test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldBeTrue)
		test.That(t, err.Error(), test.ShouldContainSubstring, unpackedModDir)
		test.That(t, err.Error(), test.ShouldContainSubstring, exeDir)

		// meta.json found in executable directory; parsing fails
		exeMetaJSONFile, err := os.Create(exeMetaJSONFilepath)
		test.That(t, err, test.ShouldBeNil)
		defer exeMetaJSONFile.Close()

		meta, moduleWorkingDirectory, err = modLocalTar.getJSONManifest(unpackedModDir, env)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, moduleWorkingDirectory, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "local tarball")
		test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldBeFalse)

		// meta.json found in executable directory; parsing succeeds
		testWriteJSON(t, exeMetaJSONFilepath, validJSONManifest)

		meta, moduleWorkingDirectory, err = modLocalTar.getJSONManifest(unpackedModDir, env)
		test.That(t, *meta, test.ShouldResemble, validJSONManifest)
		test.That(t, moduleWorkingDirectory, test.ShouldEqual, exeDir)
		test.That(t, err, test.ShouldBeNil)

		// meta.json found in unpacked modular directory; parsing fails
		unpackedModMetaJSONFile, err := os.Create(unpackedModMetaJSONFilepath)
		test.That(t, err, test.ShouldBeNil)
		defer unpackedModMetaJSONFile.Close()

		meta, moduleWorkingDirectory, err = modLocalTar.getJSONManifest(unpackedModDir, env)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, moduleWorkingDirectory, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "local tarball")
		test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldBeFalse)

		// meta.json found in unpacked module directory; parsing succeeds
		testWriteJSON(t, unpackedModMetaJSONFilepath, validJSONManifest)

		meta, moduleWorkingDirectory, err = modLocalTar.getJSONManifest(unpackedModDir, env)
		test.That(t, *meta, test.ShouldResemble, validJSONManifest)
		test.That(t, moduleWorkingDirectory, test.ShouldEqual, unpackedModDir)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("LocalNontarball", func(t *testing.T) {
		tmp := t.TempDir()

		unpackedModDir := filepath.Join(tmp, "unpacked-mod-dir")
		env := map[string]string{}
		modLocalNontar := Module{Type: ModuleTypeLocal}

		err := os.Mkdir(unpackedModDir, 0o700)
		test.That(t, err, test.ShouldBeNil)

		meta, moduleWorkingDirectory, err := modLocalNontar.getJSONManifest(unpackedModDir, env)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, moduleWorkingDirectory, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "local non-tarball")
		test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldBeFalse)
	})
}

func TestFindMetaJSONFile(t *testing.T) {
	tmp := t.TempDir()
	metaJSONFilePath := filepath.Join(tmp, "meta.json")

	t.Run("MissingMetaFile", func(t *testing.T) {
		meta, err := findMetaJSONFile(tmp)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
	})

	file, err := os.Create(metaJSONFilePath)
	test.That(t, err, test.ShouldBeNil)
	defer file.Close()
	t.Run("InvalidMetaFile", func(t *testing.T) {
		meta, err := findMetaJSONFile(tmp)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldNotEqual, os.IsNotExist)
	})

	validMeta := JSONManifest{Entrypoint: "entry"}
	testWriteJSON(t, metaJSONFilePath, &validMeta)
	t.Run("ValidMetaFileFound", func(t *testing.T) {
		meta, err := findMetaJSONFile(tmp)
		test.That(t, *meta, test.ShouldResemble, validMeta)
		test.That(t, err, test.ShouldBeNil)
	})
}

// testWriteJSON is a t.Helper that serializes `value` to `path` as json.
func testWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	file, err := os.Create(path)
	test.That(t, err, test.ShouldBeNil)
	defer file.Close()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(value)
	test.That(t, err, test.ShouldBeNil)
}
