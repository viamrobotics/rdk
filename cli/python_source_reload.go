package cli

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
)

// pythonSourceReloadEntrypoint is the UV runner copied onto the machine and used as
// reload_path for Python source hot reload (local non-tarball; no unpack).
const pythonSourceReloadEntrypoint = "run.sh"

// Extra ignore rules for Python source copy so venv / build artifacts never
// ship even when the module has no .gitignore.
var pythonSourceExtraIgnores = []string{
	".venv/",
	"venv/",
	"ENV/",
	"__pycache__/",
	"*.pyc",
	".pytest_cache/",
	".mypy_cache/",
	"dist/",
	"build/",
	"*.egg-info/",
	".VIAM_RELOAD_ARCHIVE.tar.gz",
}

func moduleSourceRoot(args reloadModuleArgs) (string, error) {
	modulePath := args.Module
	if modulePath == "" {
		modulePath = defaultManifestFilename
	}
	if args.Workdir != "" && args.Workdir != "." && !filepath.IsAbs(modulePath) {
		modulePath = filepath.Join(args.Workdir, modulePath)
	}
	abs, err := filepath.Abs(modulePath)
	if err != nil {
		return "", err
	}
	return filepath.Dir(abs), nil
}

func pythonSourceReloadDir(manifest *ModuleManifest, viamHome string) string {
	return path.Join(viamHome,
		config.PackagesDirName+config.LocalPackagesSuffix,
		utils.SanitizePath(localizeModuleID(manifest.ModuleID)))
}

func pythonSourceReloadPath(manifest *ModuleManifest, viamHome string) string {
	return path.Join(pythonSourceReloadDir(manifest, viamHome), pythonSourceReloadEntrypoint)
}

func pythonRunShPath(sourceRoot string) string {
	return filepath.Join(sourceRoot, pythonSourceReloadEntrypoint)
}

func ensurePythonRunSh(sourceRoot string) error {
	runSh := pythonRunShPath(sourceRoot)
	info, err := os.Stat(runSh)
	if err != nil {
		return fmt.Errorf("python source reload requires %s in %s (UV runner). "+
			"Add one, or pass --file with a built tarball", pythonSourceReloadEntrypoint, sourceRoot)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected the UV run.sh script", runSh)
	}
	return nil
}

type dualIgnoreMatcher struct {
	extra gitignore.Matcher
	repo  gitignore.Matcher
}

func (m dualIgnoreMatcher) Match(path []string, isDir bool) bool {
	return m.extra.Match(path, isDir) || m.repo.Match(path, isDir)
}

func (c *viamClient) pythonSourceIgnoreMatcher(repoPath string) (gitignore.Matcher, error) {
	var extras []gitignore.Pattern
	for _, pattern := range pythonSourceExtraIgnores {
		extras = append(extras, gitignore.ParsePattern(pattern, nil))
	}
	repo, err := c.loadGitignorePatterns(repoPath)
	if err != nil {
		return nil, err
	}
	return dualIgnoreMatcher{extra: gitignore.NewMatcher(extras), repo: repo}, nil
}

// gitignoreFilteredRelPaths returns gitignore-filtered files under root (files only).
func (c *viamClient) gitignoreFilteredRelPaths(root string) ([]string, error) {
	matcher, err := c.pythonSourceIgnoreMatcher(root)
	if err != nil {
		return nil, err
	}
	var files []string
	err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			if c.shouldIgnore(relPath, matcher, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if c.shouldIgnore(relPath, matcher, false) {
			return nil
		}
		files = append(files, filepath.ToSlash(relPath))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// stagePythonSourceTree copies gitignore-filtered source into destRoot, preserving
// relative paths and file modes. destRoot is created if needed.
func (c *viamClient) stagePythonSourceTree(srcRoot, destRoot string) error {
	files, err := c.gitignoreFilteredRelPaths(srcRoot)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no files to copy after applying gitignore; check the module directory")
	}
	for _, rel := range files {
		src := filepath.Join(srcRoot, filepath.FromSlash(rel))
		dst := filepath.Join(destRoot, filepath.FromSlash(rel))
		if err := copyFileWithMode(src, dst); err != nil {
			return errors.Wrapf(err, "staging %s", rel)
		}
	}
	runSh := filepath.Join(destRoot, pythonSourceReloadEntrypoint)
	if err := os.Chmod(runSh, 0o755); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "chmod run.sh")
	}
	return nil
}

func copyFileWithMode(src, dst string) (err error) {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	//nolint:gosec // src is a developer-owned module path
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Append(err, in.Close())
	}()
	//nolint:gosec // dest is a local staging directory we created
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Append(err, out.Close())
	}()
	_, err = io.Copy(out, in)
	return err
}

func preparePythonSourceStagingDir(
	c *viamClient, sourceRoot, localizedID string,
) (packagesLocalDir string, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "viam-python-reload-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }
	moduleDir := filepath.Join(tmp, config.PackagesDirName+config.LocalPackagesSuffix, localizedID)
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := c.stagePythonSourceTree(sourceRoot, moduleDir); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := ensurePythonRunSh(moduleDir); err != nil {
		cleanup()
		return "", nil, err
	}
	return filepath.Join(tmp, config.PackagesDirName+config.LocalPackagesSuffix), cleanup, nil
}

func localizedPythonModuleID(manifest *ModuleManifest) string {
	return utils.SanitizePath(localizeModuleID(manifest.ModuleID))
}
