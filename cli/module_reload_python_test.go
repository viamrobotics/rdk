package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tidwall/gjson"
	"go.viam.com/test"

	"go.viam.com/rdk/testutils/inject"
)

const pythonTemplateManifest = `{
  "module_id": "ns:mod",
  "entrypoint": "dist/main",
  "first_run": "setup_once.sh",
  "custom_field": "kept",
  "build": {
    "setup": "./setup.sh",
    "build": "./build.sh",
    "path": "dist/archive.tar.gz",
    "arch": ["linux/amd64"]
  }
}
`

// writePythonModule lays out a module like the Python template generates and returns its meta.json path.
func writePythonModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "meta.json"), pythonTemplateManifest)
	writeFile(t, filepath.Join(dir, "build.sh"), "python -m PyInstaller --onefile src/main.py")
	writeFile(t, filepath.Join(dir, "src", "main.py"), "print('hi')")
	writeFile(t, filepath.Join(dir, "src", "__pycache__", "main.cpython-313.pyc"), "bytecode")
	writeFile(t, filepath.Join(dir, "requirements.txt"), "viam-sdk")
	writeFile(t, filepath.Join(dir, "setup_once.sh"), "echo once")
	writeFile(t, filepath.Join(dir, "venv", "bin", "python"), "binary")
	return filepath.Join(dir, "meta.json")
}

func TestPackagePythonSourceReload(t *testing.T) {
	cCtx, vc, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{}, map[string]any{}, "token")

	t.Run("packages source with reload scripts and rewritten manifest", func(t *testing.T) {
		manifestPath := writePythonModule(t)
		archivePath, tmpDir, err := vc.packagePythonSourceReload(cCtx, manifestPath)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })
		test.That(t, filepath.Base(archivePath), test.ShouldEqual, pythonReloadArchiveName)

		entries := archiveEntries(t, archivePath)
		test.That(t, sortedKeys(entries), test.ShouldResemble, []string{
			".user_first_run", "build.sh", "meta.json", pythonReloadFirstRunName, pythonReloadRunName,
			"requirements.txt", "setup_once.sh", "src/main.py",
		})
		test.That(t, entries[pythonReloadFirstRunName].content, test.ShouldEqual, string(pythonReloadFirstRun))
		test.That(t, entries[pythonReloadFirstRunName].mode, test.ShouldEqual, int64(0o755))
		test.That(t, entries[pythonReloadRunName].mode, test.ShouldEqual, int64(0o755))
		test.That(t, entries[".user_first_run"].content, test.ShouldEqual, "setup_once.sh")

		meta := entries["meta.json"].content
		test.That(t, gjson.Get(meta, "entrypoint").String(), test.ShouldEqual, pythonReloadRunName)
		test.That(t, gjson.Get(meta, "first_run").String(), test.ShouldEqual, pythonReloadFirstRunName)
		test.That(t, gjson.Get(meta, "custom_field").String(), test.ShouldEqual, "kept")
	})

	t.Run("missing first_run script is not chained", func(t *testing.T) {
		manifestPath := writePythonModule(t)
		test.That(t, os.Remove(filepath.Join(filepath.Dir(manifestPath), "setup_once.sh")), test.ShouldBeNil)
		archivePath, tmpDir, err := vc.packagePythonSourceReload(cCtx, manifestPath)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })
		_, ok := archiveEntries(t, archivePath)[".user_first_run"]
		test.That(t, ok, test.ShouldBeFalse)
	})

	t.Run("pyproject.toml satisfies the dependency requirement", func(t *testing.T) {
		manifestPath := writePythonModule(t)
		dir := filepath.Dir(manifestPath)
		test.That(t, os.Remove(filepath.Join(dir, "requirements.txt")), test.ShouldBeNil)
		writeFile(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname = \"mod\"\n")
		_, tmpDir, err := vc.packagePythonSourceReload(cCtx, manifestPath)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.RemoveAll(tmpDir) })
	})

	t.Run("requires dependencies file", func(t *testing.T) {
		manifestPath := writePythonModule(t)
		test.That(t, os.Remove(filepath.Join(filepath.Dir(manifestPath), "requirements.txt")), test.ShouldBeNil)
		_, _, err := vc.packagePythonSourceReload(cCtx, manifestPath)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "requirements.txt or pyproject.toml")
	})

	t.Run("requires src/main.py", func(t *testing.T) {
		manifestPath := writePythonModule(t)
		test.That(t, os.Remove(filepath.Join(filepath.Dir(manifestPath), "src", "main.py")), test.ShouldBeNil)
		_, _, err := vc.packagePythonSourceReload(cCtx, manifestPath)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "src/main.py")
	})
}

func TestLooksLikePythonTemplateModule(t *testing.T) {
	load := func(t *testing.T, manifestPath string) *ModuleManifest {
		t.Helper()
		manifest, err := loadManifestOrNil(manifestPath)
		test.That(t, err, test.ShouldBeNil)
		return manifest
	}

	t.Run("template module", func(t *testing.T) {
		manifestPath := writePythonModule(t)
		test.That(t, looksLikePythonTemplateModule(manifestPath, load(t, manifestPath)), test.ShouldBeTrue)
	})

	t.Run("build script without PyInstaller", func(t *testing.T) {
		manifestPath := writePythonModule(t)
		writeFile(t, filepath.Join(filepath.Dir(manifestPath), "build.sh"), "go build ./...")
		test.That(t, looksLikePythonTemplateModule(manifestPath, load(t, manifestPath)), test.ShouldBeFalse)
	})

	t.Run("different build command", func(t *testing.T) {
		manifestPath := writePythonModule(t)
		manifest := load(t, manifestPath)
		manifest.Build.Build = "make module.tar.gz"
		test.That(t, looksLikePythonTemplateModule(manifestPath, manifest), test.ShouldBeFalse)
	})

	t.Run("no manifest", func(t *testing.T) {
		test.That(t, looksLikePythonTemplateModule("meta.json", nil), test.ShouldBeFalse)
	})
}

func TestSetReloadFromSource(t *testing.T) {
	manifestPath := writePythonModule(t)

	test.That(t, setReloadFromSource(manifestPath, false), test.ShouldBeNil)
	test.That(t, setReloadFromSource(manifestPath, true), test.ShouldBeNil)

	//nolint:gosec // test temp path
	raw, err := os.ReadFile(manifestPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, json.Valid(raw), test.ShouldBeTrue)
	test.That(t, gjson.GetBytes(raw, "build.reload_from_source").Bool(), test.ShouldBeTrue)
	test.That(t, gjson.GetBytes(raw, "custom_field").String(), test.ShouldEqual, "kept")
	test.That(t, gjson.GetBytes(raw, "build.path").String(), test.ShouldEqual, "dist/archive.tar.gz")

	manifest, err := loadManifestOrNil(manifestPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reloadsFromSource(manifest), test.ShouldBeTrue)
}

func TestOfferReloadFromSourceNonInteractive(t *testing.T) {
	cCtx, _, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{}, map[string]any{}, "token")
	manifestPath := writePythonModule(t)
	manifest, err := loadManifestOrNil(manifestPath)
	test.That(t, err, test.ShouldBeNil)

	enabled, err := offerReloadFromSource(cCtx, manifestPath, manifest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, enabled, test.ShouldBeFalse)

	//nolint:gosec // test temp path
	raw, err := os.ReadFile(manifestPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(raw), test.ShouldEqual, pythonTemplateManifest)
}
