package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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

func TestFindMetaJSONFile(t *testing.T) {
	tmp := t.TempDir()
	metaJSONFilePath := filepath.Join(tmp, "meta.json")

	t.Run("MissingMetaFile", func(t *testing.T) {
		meta, err := findMetaJSONFile(tmp)
		test.That(t, meta, test.ShouldBeNil)
		test.That(t, err, test.ShouldEqual, os.IsNotExist)
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
		test.That(t, meta, test.ShouldEqual, validMeta)
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
