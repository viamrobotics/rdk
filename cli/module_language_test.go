package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"go.viam.com/test"
)

func TestNormalizeModuleLanguage(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"python": moduleLanguagePython,
		"PYTHON": moduleLanguagePython,
		"golang": moduleLanguageGolang,
		"go":     moduleLanguageGolang,
		"Go":     moduleLanguageGolang,
		"cpp":    moduleLanguageCPP,
		"c++":    moduleLanguageCPP,
		"":       "",
		"rust":   "",
		"  go  ": moduleLanguageGolang,
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			test.That(t, NormalizeModuleLanguage(raw), test.ShouldEqual, want)
		})
	}
}

func TestParseModuleLanguageFlag(t *testing.T) {
	t.Parallel()
	got, err := parseModuleLanguageFlag("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, "")

	got, err = parseModuleLanguageFlag("python")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, moduleLanguagePython)

	_, err = parseModuleLanguageFlag("javascript")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, moduleFlagLanguage)
}

func TestInferModuleLanguage(t *testing.T) {
	t.Parallel()

	t.Run("python from requirements", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "requirements.txt"), "viam-sdk\n")
		test.That(t, InferModuleLanguage(root), test.ShouldEqual, moduleLanguagePython)
	})

	t.Run("python from src/main.py", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "src", "main.py"), "print('hi')\n")
		test.That(t, InferModuleLanguage(root), test.ShouldEqual, moduleLanguagePython)
	})

	t.Run("golang from go.mod", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "go.mod"), "module example.com/mod\n")
		test.That(t, InferModuleLanguage(root), test.ShouldEqual, moduleLanguageGolang)
	})

	t.Run("cpp from CMakeLists", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "CMakeLists.txt"), "project(mod)\n")
		test.That(t, InferModuleLanguage(root), test.ShouldEqual, moduleLanguageCPP)
	})

	t.Run("ambiguous python+go", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "go.mod"), "module example.com/mod\n")
		writeFile(t, filepath.Join(root, "requirements.txt"), "viam-sdk\n")
		test.That(t, InferModuleLanguage(root), test.ShouldEqual, "")
	})

	t.Run("empty tree", func(t *testing.T) {
		t.Parallel()
		test.That(t, InferModuleLanguage(t.TempDir()), test.ShouldEqual, "")
	})
}

func TestResolveModuleLanguagePrecedence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/mod\n")
	manifest := &ModuleManifest{Language: "python"}

	got, err := resolveModuleLanguage(manifest, "cpp", root)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, moduleLanguageCPP)

	got, err = resolveModuleLanguage(manifest, "", root)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, moduleLanguagePython)

	got, err = resolveModuleLanguage(&ModuleManifest{}, "", root)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, moduleLanguageGolang)
}

func TestPythonSourceReloadPath(t *testing.T) {
	t.Parallel()
	manifest := &ModuleManifest{ModuleID: "viam-labs:test-module"}
	test.That(t, pythonSourceReloadDir(manifest, "/opt/viam"),
		test.ShouldEqual, "/opt/viam/packages-local/viam-labs_test-module")
	test.That(t, pythonSourceReloadPath(manifest, "/opt/viam"),
		test.ShouldEqual, "/opt/viam/packages-local/viam-labs_test-module/run.sh")
	test.That(t, pythonSourceReloadPath(manifest, legacyViamHomeDir),
		test.ShouldEqual, "~/.viam/packages-local/viam-labs_test-module/run.sh")
}

func TestGitignoreFilteredPythonSourceTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".gitignore"), "venv/\nignore_me.py\n")
	writeFile(t, filepath.Join(root, "run.sh"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(root, "meta.json"), "{}\n")
	writeFile(t, filepath.Join(root, "requirements.txt"), "viam-sdk\n")
	writeFile(t, filepath.Join(root, "src", "main.py"), "print(1)\n")
	writeFile(t, filepath.Join(root, "src", "models", "generic.py"), "pass\n")
	writeFile(t, filepath.Join(root, "venv", "lib", "foo.py"), "ignored\n")
	writeFile(t, filepath.Join(root, "ignore_me.py"), "nope\n")
	writeFile(t, filepath.Join(root, ".venv", "x.py"), "ignored extra\n")

	vc := &viamClient{}
	files, err := vc.gitignoreFilteredRelPaths(root)
	test.That(t, err, test.ShouldBeNil)
	sort.Strings(files)
	test.That(t, files, test.ShouldResemble, []string{
		".gitignore",
		"meta.json",
		"requirements.txt",
		"run.sh",
		"src/main.py",
		"src/models/generic.py",
	})

	dest := t.TempDir()
	test.That(t, vc.stagePythonSourceTree(root, dest), test.ShouldBeNil)
	test.That(t, fileExists(filepath.Join(dest, "src", "main.py")), test.ShouldBeTrue)
	test.That(t, fileExists(filepath.Join(dest, "venv", "lib", "foo.py")), test.ShouldBeFalse)
	info, err := os.Stat(filepath.Join(dest, "run.sh"))
	test.That(t, err, test.ShouldBeNil)
	if runtime.GOOS != "windows" {
		test.That(t, info.Mode().Perm()&0o111, test.ShouldNotEqual, os.FileMode(0))
	}
}

func TestEnsurePythonRunSh(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	err := ensurePythonRunSh(root)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "run.sh")

	writeFile(t, filepath.Join(root, "run.sh"), "#!/bin/sh\n")
	test.That(t, ensurePythonRunSh(root), test.ShouldBeNil)
}

func TestIsPythonSourceHotReload(t *testing.T) {
	t.Parallel()
	test.That(t, isPythonSourceHotReload(moduleLanguagePython), test.ShouldBeTrue)
	test.That(t, isPythonSourceHotReload(moduleLanguageGolang), test.ShouldBeFalse)
	test.That(t, isPythonSourceHotReload(moduleLanguageCPP), test.ShouldBeFalse)
	test.That(t, isPythonSourceHotReload(""), test.ShouldBeFalse)
}

func TestPreparePythonSourceStagingDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "run.sh"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(root, "meta.json"), "{}\n")
	writeFile(t, filepath.Join(root, "src", "main.py"), "print(1)\n")
	writeFile(t, filepath.Join(root, ".venv", "x.py"), "ignored\n")

	vc := &viamClient{}
	staging, cleanup, err := preparePythonSourceStagingDir(vc, root, "acme_mod")
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(cleanup)

	test.That(t, fileExists(filepath.Join(staging, "acme_mod", "run.sh")), test.ShouldBeTrue)
	test.That(t, fileExists(filepath.Join(staging, "acme_mod", "src", "main.py")), test.ShouldBeTrue)
	test.That(t, fileExists(filepath.Join(staging, "acme_mod", ".venv", "x.py")), test.ShouldBeFalse)
}
