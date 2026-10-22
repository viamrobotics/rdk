package cli

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
	"github.com/urfave/cli/v3"
)

//go:embed python_reload/first_run.sh
var pythonReloadFirstRun []byte

//go:embed python_reload/run.sh
var pythonReloadRun []byte

const (
	pythonReloadFirstRunName = "reload_first_run.sh"
	pythonReloadRunName      = "reload_run.sh"
	pythonReloadArchiveName  = "python-source.tar.gz"
)

// pythonReloadIgnores keep local environments and build output out of the source archive even
// when the module's .gitignore doesn't list them.
var pythonReloadIgnores = []string{"venv/", ".venv/", "__pycache__/", "*.pyc", "dist/", "build/", "*.egg-info/"}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// reloadsFromSource reports whether the module opted in to running from source on reload.
func reloadsFromSource(manifest *ModuleManifest) bool {
	return manifest != nil && manifest.Build != nil &&
		manifest.Build.ReloadFromSource != nil && *manifest.Build.ReloadFromSource
}

// packagePythonSourceReload writes a tarball of the module's source, plus scripts that install
// its dependencies with uv and run src/main.py, to a new temp directory. The caller removes the
// returned directory when done.
func (c *viamClient) packagePythonSourceReload(cmd *cli.Command, manifestPath string) (archivePath, tmpDir string, err error) {
	moduleDir := filepath.Dir(manifestPath)
	if !fileExists(filepath.Join(moduleDir, "src", "main.py")) {
		return "", "", errors.Errorf("reload_from_source requires src/main.py in %s", moduleDir)
	}
	if !fileExists(filepath.Join(moduleDir, "requirements.txt")) && !fileExists(filepath.Join(moduleDir, "pyproject.toml")) {
		return "", "", errors.Errorf("reload_from_source requires a requirements.txt or pyproject.toml in %s", moduleDir)
	}

	//nolint:gosec // manifestPath is the user's own meta.json
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", "", err
	}
	userFirstRun := gjson.GetBytes(rawManifest, "first_run").String()
	rawManifest, err = sjson.SetBytes(rawManifest, "entrypoint", pythonReloadRunName)
	if err != nil {
		return "", "", err
	}
	rawManifest, err = sjson.SetBytes(rawManifest, "first_run", pythonReloadFirstRunName)
	if err != nil {
		return "", "", err
	}

	injected := []injectedFile{
		{name: defaultManifestFilename, content: rawManifest},
		{name: pythonReloadFirstRunName, content: pythonReloadFirstRun, mode: 0o755},
		{name: pythonReloadRunName, content: pythonReloadRun, mode: 0o755},
	}
	if userFirstRun != "" {
		if fileExists(filepath.Join(moduleDir, userFirstRun)) {
			// reload_first_run.sh runs the module's own first_run script after installing dependencies.
			injected = append(injected, injectedFile{name: ".user_first_run", content: []byte(userFirstRun)})
		} else {
			warningf(cmd.Root().ErrWriter, "first_run script %q declared in meta.json was not found; it will not run", userFirstRun)
		}
	}

	tmpDir, err = os.MkdirTemp("", "viam-python-reload-*")
	if err != nil {
		return "", "", err
	}
	archivePath = filepath.Join(tmpDir, pythonReloadArchiveName)
	if err := c.writeSourceArchive(moduleDir, archivePath, pythonReloadIgnores, injected); err != nil {
		_ = os.RemoveAll(tmpDir) //nolint:errcheck
		return "", "", err
	}
	return archivePath, tmpDir, nil
}

// looksLikePythonTemplateModule reports whether a module appears to come from the Python template
// of `viam module generate` and builds with PyInstaller, so reloading from source should work.
func looksLikePythonTemplateModule(manifestPath string, manifest *ModuleManifest) bool {
	if manifest == nil || manifest.Build == nil || manifest.Build.Build != "./build.sh" {
		return false
	}
	moduleDir := filepath.Dir(manifestPath)
	//nolint:gosec // reading the module's own build script
	buildScript, err := os.ReadFile(filepath.Join(moduleDir, "build.sh"))
	if err != nil || !strings.Contains(string(buildScript), "PyInstaller") {
		return false
	}
	return fileExists(filepath.Join(moduleDir, "src", "main.py"))
}

// offerReloadFromSource asks whether to enable reload_from_source for a module that looks like
// it could use it, and records the answer in meta.json so the question is asked only once.
// Returns whether reload_from_source is now enabled.
func offerReloadFromSource(cmd *cli.Command, manifestPath string, manifest *ModuleManifest) (bool, error) {
	const hint = `this module can reload faster by running from source instead of building with PyInstaller. ` +
		`Set "reload_from_source": true in the "build" section of meta.json to enable it`
	if !isInteractive() {
		infof(cmd.Root().ErrWriter, "%s", hint)
		return false, nil
	}

	enable := true
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Reload faster by running from source?").
			Description("Reloads currently build this module with PyInstaller. With reload_from_source,\n" +
				"the CLI sends the source to the machine instead, where dependencies are installed\n" +
				"with uv and cached between reloads. Released versions still use PyInstaller.\n\n" +
				`This adds "reload_from_source" to the "build" section of meta.json.`).
			Affirmative("Enable").
			Negative("Not now").
			Value(&enable),
	))
	if err := form.Run(); err != nil {
		return false, errors.Wrap(err, "reload_from_source prompt")
	}
	if err := setReloadFromSource(manifestPath, enable); err != nil {
		return false, err
	}
	manifest.Build.ReloadFromSource = &enable
	if !enable {
		infof(cmd.Root().ErrWriter, `saved "reload_from_source": false to meta.json; set it to true to enable later`)
	}
	return enable, nil
}

// setReloadFromSource sets build.reload_from_source in meta.json without dropping fields the CLI
// doesn't model, re-indenting the file with two spaces.
func setReloadFromSource(manifestPath string, enable bool) error {
	//nolint:gosec // manifestPath is the user's own meta.json
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	raw, err = sjson.SetBytes(raw, "build.reload_from_source", enable)
	if err != nil {
		return err
	}
	raw = pretty.PrettyOptions(raw, &pretty.Options{Indent: "  "})
	info, err := os.Stat(manifestPath)
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, raw, info.Mode().Perm())
}
