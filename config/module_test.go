package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
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

		err := os.Mkdir(unpackedModDir, 0700)

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
		env["VIAM_MODULE_ROOT"] = tmp

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
		// meta.json found in top level modular directory; parsing succeeds
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
