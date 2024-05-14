package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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
		pkg, err := modNeedsSynthetic.SyntheticPackage(false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pkg.Version, test.ShouldEqual, "0.0.0")
		_, err = modNotLocal.SyntheticPackage(false)
		test.That(t, err, test.ShouldNotBeNil)

		tmp := t.TempDir()
		f, err := os.Create(filepath.Join(tmp, "synthetic-package-checks-me"))
		test.That(t, err, test.ShouldBeNil)
		err = f.Close()
		test.That(t, err, test.ShouldBeNil)
		pkg, err = (Module{Type: ModuleTypeLocal, ExePath: f.Name()}).SyntheticPackage(true)
		test.That(t, err, test.ShouldBeNil)
		match, err := regexp.MatchString(`0\.0\.0-\d+-\d+`, pkg.Version)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, match, test.ShouldBeTrue)
	})

	t.Run("syntheticPackageExeDir", func(t *testing.T) {
		dir, err := modNeedsSynthetic.syntheticPackageExeDir(false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dir, test.ShouldEqual, filepath.Join(viamPackagesDir, "data/module/synthetic--0_0_0"))
	})

	t.Run("EvaluateExePath", func(t *testing.T) {
		meta := EntrypointOnlyMetaJSON{
			Entrypoint: "entry",
		}
		testWriteJSON(t, filepath.Join(tmp, "meta.json"), &meta)
		syntheticPath, err := modNeedsSynthetic.EvaluateExePath(false)
		test.That(t, err, test.ShouldBeNil)
		exeDir, err := modNeedsSynthetic.syntheticPackageExeDir(false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, syntheticPath, test.ShouldEqual, filepath.Join(exeDir, meta.Entrypoint))
		notTarPath, err := modNotTar.EvaluateExePath(false)
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
