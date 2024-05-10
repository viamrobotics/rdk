package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

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
		dir, err := modNeedsSynthetic.syntheticPackageExeDir()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dir, test.ShouldEqual, filepath.Join(viamPackagesDir, "data/module/synthetic--"))
	})

	t.Run("EvaluateExePath", func(t *testing.T) {
		meta := EntrypointOnlyMetaJSON{
			Entrypoint: "entry",
		}
		testWriteJSON(t, filepath.Join(tmp, "meta.json"), &meta)
		syntheticPath, err := modNeedsSynthetic.EvaluateExePath()
		test.That(t, err, test.ShouldBeNil)
		exeDir, err := modNeedsSynthetic.syntheticPackageExeDir()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, syntheticPath, test.ShouldEqual, filepath.Join(exeDir, meta.Entrypoint))
		notTarPath, err := modNotTar.EvaluateExePath()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, notTarPath, test.ShouldEqual, modNotTar.ExePath)
	})
}

func testWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	file, err := os.Create(path)
	test.That(t, err, test.ShouldBeNil)
	defer file.Close()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(value)
	test.That(t, err, test.ShouldBeNil)
}