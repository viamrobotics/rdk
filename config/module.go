package config

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

const (
	reservedModuleName     = "parent"
	defaultFirstRunTimeout = 1 * time.Hour
)

// Module represents an external resource module, with a path to the binary module file.
type Module struct {
	// Name is an arbitrary name used to identify the module, and is used to name it's socket as well.
	Name string `json:"name"`
	// ExePath is the path (either absolute, or relative to the working directory) to the executable module file.
	ExePath string `json:"executable_path"`
	// LogLevel represents the level at which the module should log its messages. It will be passed as a commandline
	// argument "log-level" (i.e. preceded by "--log-level=") to the module executable. If unset or set to an empty
	// string, "--log-level=debug" will be passed to the module executable if the server was started with "-debug".
	//
	// SDK logger-creation utilities, such as module.NewLoggerFromArgs, will create an "Info" level logger when any
	// value besides "" or "debug" is used for LogLevel ("log_level" in JSON). In other words, setting a LogLevel
	// of something like "info" will ignore the debug setting on the server.
	LogLevel string `json:"log_level"`
	// Type indicates whether this is a local or registry module.
	Type ModuleType `json:"type"`
	// ModuleID is the id of the module in the registry. It is empty on non-registry modules.
	ModuleID string `json:"module_id,omitempty"`
	// Environment contains additional variables that are passed to the module process when it is started.
	// They overwrite existing environment variables.
	Environment map[string]string `json:"env,omitempty"`

	// FirstRunTimeout is the timeout duration for the first run script.
	// This field will only be applied if it is a positive value. Supplying a
	// non-positive will set the first run timeout to the default value of 1 hour,
	// which is equivalent to leaving this field unset. If you wish to set an
	// immediate timeout you should set this field to a very small positive value
	// such as "1ns".
	FirstRunTimeout goutils.Duration `json:"first_run_timeout,omitempty"`

	// Status refers to the validations done in the APP to make sure a module is configured correctly
	Status           *AppValidationStatus `json:"status"`
	alreadyValidated bool
	cachedErr        error

	// LocalVersion is an in-process fake version used for local module change management.
	LocalVersion string
}

// JSONManifest contains meta.json fields that are used by both RDK and CLI.
type JSONManifest struct {
	Entrypoint string `json:"entrypoint"`
	FirstRun   string `json:"first_run"`
}

// ModuleType indicates where a module comes from.
type ModuleType string

const (
	// ModuleTypeLocal is a module that resides on the host system.
	ModuleTypeLocal ModuleType = "local"
	// ModuleTypeRegistry is a module from our registry that is distributed in a package and is downloaded at runtime.
	ModuleTypeRegistry ModuleType = "registry"
)

// Validate checks if the config is valid.
func (m *Module) Validate(path string) error {
	if m.alreadyValidated {
		return m.cachedErr
	}
	if m.Status != nil {
		m.alreadyValidated = true
		m.cachedErr = resource.NewConfigValidationError(path, errors.New(m.Status.Error))
		return m.cachedErr
	}
	m.cachedErr = m.validate(path)
	m.alreadyValidated = true
	return m.cachedErr
}

func (m *Module) validate(path string) error {
	// Only check if the path exists during validation for local modules because the packagemanager may not have downloaded
	// the package yet.
	if m.Type == ModuleTypeLocal {
		_, err := os.Stat(m.ExePath)
		if err != nil {
			return fmt.Errorf("module %s executable path error: %w", path, err)
		}
	}

	if err := utils.ValidateModuleName(m.Name); err != nil {
		return resource.NewConfigValidationError(path, err)
	}

	if m.Name == reservedModuleName {
		return fmt.Errorf("module %s cannot use the reserved name of %s", path, reservedModuleName)
	}

	return nil
}

// Equals checks if the two modules are deeply equal to each other.
func (m Module) Equals(other Module) bool {
	m.alreadyValidated = false
	m.cachedErr = nil
	m.Status = nil
	other.alreadyValidated = false
	other.cachedErr = nil
	other.Status = nil
	//nolint:govet
	return reflect.DeepEqual(m, other)
}

// MergeEnvVars will merge the provided environment variables with the existing Environment, with the existing Environment
// taking priority.
func (m *Module) MergeEnvVars(env map[string]string) {
	if m.Environment == nil {
		m.Environment = make(map[string]string)
	}
	for k, v := range env {
		if _, ok := m.Environment[k]; ok {
			continue
		}
		m.Environment[k] = v
	}
}

var tarballExtensionsRegexp = regexp.MustCompile(`\.(tgz|tar\.gz)$`)

// NeedsSyntheticPackage returns true if this is a local module pointing at a tarball.
func (m Module) NeedsSyntheticPackage() bool {
	return m.Type == ModuleTypeLocal && tarballExtensionsRegexp.MatchString(strings.ToLower(m.ExePath))
}

// SyntheticPackage creates a fake package for a module which can be used to access some package logic.
func (m Module) SyntheticPackage() (PackageConfig, error) {
	if m.Type != ModuleTypeLocal {
		return PackageConfig{}, errors.New("SyntheticPackage only works on local modules")
	}
	var name string
	if m.NeedsSyntheticPackage() {
		name = fmt.Sprintf("synthetic-%s", m.Name)
	} else {
		name = m.Name
	}
	return PackageConfig{Name: name, Package: name, Type: PackageTypeModule, Version: m.LocalVersion}, nil
}

// exeDir returns the parent directory for the unpacked module.
func (m Module) exeDir(packagesDir string) (string, error) {
	if !m.NeedsSyntheticPackage() {
		return filepath.Dir(m.ExePath), nil
	}
	pkg, err := m.SyntheticPackage()
	if err != nil {
		return "", err
	}
	return pkg.LocalDataDirectory(packagesDir), nil
}

// parseJSONFile returns a *T by parsing the json file at `path`.
func parseJSONFile[T any](path string) (*T, error) {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("reading json file: %w", err)
	}
	var target T
	err = json.NewDecoder(f).Decode(&target)
	if err != nil {
		return nil, err
	}
	return &target, nil
}

// EvaluateExePath returns absolute ExePath from one of three sources (in order of precedence):
// 1. if there is a meta.json in the exe dir, use that, except in local non-tarball case.
// 2. if this is a local tarball and there's a meta.json next to the tarball, use that.
// 3. otherwise use the exe path from config, or fail if this is a local tarball.
// Note: the working directory must be the unpacked tarball directory or local exec directory.
func (m Module) EvaluateExePath(packagesDir string) (string, error) {
	if !filepath.IsAbs(m.ExePath) {
		return "", fmt.Errorf("expected ExePath to be absolute path, got %s", m.ExePath)
	}
	exeDir, err := m.exeDir(packagesDir)
	if err != nil {
		return "", err
	}
	// note: we don't look at internal meta.json in local non-tarball case because user has explicitly requested a binary.
	localNonTarball := m.Type == ModuleTypeLocal && !m.NeedsSyntheticPackage()
	if !localNonTarball {
		// this is case 1, meta.json in exe folder.
		metaPath, err := utils.SafeJoinDir(exeDir, "meta.json")
		if err != nil {
			return "", err
		}
		_, err = os.Stat(metaPath)
		if err == nil {
			// this is case 1, meta.json in exe dir
			meta, err := parseJSONFile[JSONManifest](metaPath)
			if err != nil {
				return "", err
			}
			entrypoint, err := utils.SafeJoinDir(exeDir, meta.Entrypoint)
			if err != nil {
				return "", err
			}
			return filepath.Abs(entrypoint)
		}
	}
	if m.NeedsSyntheticPackage() {
		// this is case 2, side-by-side
		// TODO(RSDK-7848): remove this case once java sdk supports internal meta.json.
		metaPath, err := utils.SafeJoinDir(filepath.Dir(m.ExePath), "meta.json")
		if err != nil {
			return "", err
		}
		meta, err := parseJSONFile[JSONManifest](metaPath)
		if err != nil {
			// note: this error deprecates the side-by-side case because the side-by-side case is deprecated.
			return "", fmt.Errorf("couldn't find meta.json inside tarball %s (or next to it): %w", m.ExePath, err)
		}
		entrypoint, err := utils.SafeJoinDir(exeDir, meta.Entrypoint)
		if err != nil {
			return "", err
		}
		return filepath.Abs(entrypoint)
	}
	return m.ExePath, nil
}

// FirstRunSuccessSuffix is the suffix of the file whose existence
// denotes that the first run script for a module ran successfully.
//
// Note that we create a new file instead of writing to `.status.json`,
// which contains various package/module state tracking information.
// Writing to `.status.json` introduces the risk of corrupting it, which
// could break or uncoordinate package sync.
const FirstRunSuccessSuffix = ".first_run_succeeded"

// FirstRun executes a module-specific setup script.
func (m *Module) FirstRun(
	ctx context.Context,
	localPackagesDir,
	dataDir string,
	env map[string]string,
	logger logging.Logger,
) error {
	logger = logger.Sublogger("first_run").WithFields("module", m.Name)

	unpackedModDir, err := m.exeDir(localPackagesDir)
	if err != nil {
		return err
	}

	// check if FirstRun already ran successfully for this module version by checking if a success
	// marker file exists on disk. An example module directory structure:
	//
	// .viam/packages/data/module/e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb-viamrtsp-0_1_0-linux-amd64/
	// .viam/packages/data/module/e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb-viamrtsp-0_1_0-linux-amd64/bin/
	// .viam/packages/data/module/e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb-viamrtsp-0_1_0-linux-amd64/bin.first_run_succeeded
	// .viam/packages/data/module/e76d1b3b-0468-4efd-bb7f-fb1d2b352fcb-viamrtsp-0_1_0-linux-amd64/bin/viamrtsp
	firstRunSuccessPath := unpackedModDir + FirstRunSuccessSuffix
	if _, err := os.Stat(firstRunSuccessPath); !errors.Is(err, os.ErrNotExist) {
		logger.Info("first run already ran")
		return nil
	}

	// Load the module's meta.json. If it doesn't exist DEBUG log and exit quietly.
	// For all other errors WARN log and exit.
	meta, moduleWorkingDirectory, err := m.getJSONManifest(unpackedModDir, env)
	var pathErr *os.PathError
	switch {
	case errors.As(err, &pathErr):
		logger.Infow("meta.json does not exist, skipping first run")
		return nil
	case err != nil:
		logger.Warnw("failed to parse meta.json, skipping first run", "error", err)
		return nil
	}

	if meta.FirstRun == "" {
		logger.Debug("no first run script specified, skipping first run")
		return nil
	}
	relFirstRunPath, err := utils.SafeJoinDir(moduleWorkingDirectory, meta.FirstRun)
	if err != nil {
		logger.Errorw("failed to build path to first run script, skipping first run", "error", err)
		return nil
	}
	firstRunPath, err := filepath.Abs(relFirstRunPath)
	if err != nil {
		logger.Errorw("failed to build absolute path to first run script, skipping first run", "path", relFirstRunPath, "error", err)
		return nil
	}

	logger = logger.WithFields("module", m.Name, "path", firstRunPath)
	logger.Infow("executing first run script")

	timeout := defaultFirstRunTimeout
	if m.FirstRunTimeout > 0 {
		timeout = m.FirstRunTimeout.Unwrap()
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	//nolint:gosec // Yes, we are deliberating executing arbitrary user code here.
	cmd := exec.CommandContext(cmdCtx, firstRunPath)

	cmd.Env = os.Environ()
	for key, val := range env {
		cmd.Env = append(cmd.Env, key+"="+val)
	}
	utils.LogViamEnvVariables("Running first run script with following Viam environment variables", env, logger)

	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	scanOut := bufio.NewScanner(stdOut)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()

		for scanOut.Scan() {
			logger.Infow("got stdio", "output", scanOut.Text())
		}
		// This scanner keeps trying to read stdio until the command terminates,
		// at which point the stdio pipe handle is no longer available. This sometimes
		// results in an `os.ErrClosed`, which we discard.
		if err := scanOut.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
			logger.Errorw("error scanning stdio", "error", err)
		}
	}()
	scanErr := bufio.NewScanner(stdErr)
	go func() {
		defer wg.Done()

		for scanErr.Scan() {
			logger.Warnw("got stderr", "output", scanErr.Text())
		}
		// This scanner keeps trying to read stderr until the command terminates,
		// at which point the stderr pipe handle is no longer available. This sometimes
		// results in an `os.ErrClosed`, which we discard.
		if err := scanErr.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
			logger.Errorw("error scanning stderr", "error", err)
		}
	}()
	if err := cmd.Start(); err != nil {
		logger.Errorw("failed to start first run script", "error", err)
		return err
	}
	if err := cmd.Wait(); err != nil {
		logger.Errorw("first run script failed", "error", err)
		return err
	}
	wg.Wait()
	logger.Info("first run script succeeded")

	// Mark success by writing a marker file to disk. This is a best
	// effort; if writing to disk fails the first run script will run again
	// for this module and version and we are okay with that.
	//nolint:gosec // safe
	markerFile, err := os.Create(firstRunSuccessPath)
	if err != nil {
		logger.Errorw("failed to mark success", "error", err)
		return nil
	}
	if err = markerFile.Close(); err != nil {
		logger.Errorw("failed to close marker file", "error", err)
		return nil
	}
	return nil
}

// getJSONManifest returns a loaded meta.json from one of three sources (in order of precedence):
// 1. if this is a registry module and there is a meta.json in its top level directory, use that.
// 2. if there is a meta.json in the exe dir, use that, except in local non-tarball case.
// 3. if this is a local tarball, use the meta.json in unpackedModDir.
// Note: the working directory must be the unpacked tarball directory or local exec directory.
func (m Module) getJSONManifest(unpackedModDir string, env map[string]string) (*JSONManifest, string, error) {
	// note: all registry modules qualify for cases 1 & 2; local tarballs for cases 2 & 3; and local non-tarballs for none. We don't look at
	// internal meta.json in local non-tarball case because user has explicitly requested a binary.

	// note: each case is exited iff no errors occur but the meta.json file is not found

	var ok bool
	var moduleWorkingDirectory string
	var registryErr error

	online := m.Type == ModuleTypeRegistry

	// case 1: registry
	if online {
		moduleWorkingDirectory, ok = env["VIAM_MODULE_ROOT"]
		if ok {
			var meta *JSONManifest
			meta, registryErr = findMetaJSONFile(moduleWorkingDirectory)
			if registryErr != nil {
				// return from getJSONManifest() if the error returned does NOT indicate that the file wasn't found
				if !os.IsNotExist(registryErr) {
					return nil, "", fmt.Errorf("registry module: %w", registryErr)
				}
			}

			if meta != nil {
				return meta, moduleWorkingDirectory, nil
			}
		}
	}

	var registryTarballErr error

	localNonTarball := m.Type == ModuleTypeLocal && !m.NeedsSyntheticPackage()

	// case 2: registry OR tarball
	if !localNonTarball && unpackedModDir != moduleWorkingDirectory {
		var meta *JSONManifest
		meta, registryTarballErr = findMetaJSONFile(unpackedModDir)
		if registryTarballErr != nil {
			if !os.IsNotExist(registryTarballErr) {
				if online {
					return nil, "", fmt.Errorf("registry module: %w", registryTarballErr)
				}

				return nil, "", fmt.Errorf("local tarball: %w", registryTarballErr)
			}
		}

		if meta != nil {
			return meta, unpackedModDir, nil
		}
	}

	var exeDir string
	var localTarballErr error

	// TODO(RSDK-7848): remove this case once java sdk supports internal meta.json.
	// case 3: local AND tarball
	if m.NeedsSyntheticPackage() {
		exeDir = filepath.Dir(m.ExePath)

		var meta *JSONManifest
		meta, localTarballErr = findMetaJSONFile(exeDir)
		if localTarballErr != nil {
			if !os.IsNotExist(localTarballErr) {
				return nil, "", fmt.Errorf("local tarball: %w", localTarballErr)
			}
		}

		if meta != nil {
			return meta, exeDir, nil
		}
	}

	if online {
		if !ok {
			return nil, "", fmt.Errorf("registry module: failed to find meta.json. VIAM_MODULE_ROOT not set: %w", registryTarballErr)
		}

		return nil, "", fmt.Errorf("registry module: failed to find meta.json: %w", errors.Join(registryErr, registryTarballErr))
	}

	if !localNonTarball {
		return nil, "", fmt.Errorf("local tarball: failed to find meta.json: %w", errors.Join(registryTarballErr, localTarballErr))
	}

	return nil, "", errors.New("local non-tarball: did not search for meta.json")
}

func findMetaJSONFile(dir string) (*JSONManifest, error) {
	metaPath, err := utils.SafeJoinDir(dir, "meta.json")
	if err != nil {
		return nil, err
	}

	_, err = os.Stat(metaPath)
	if err != nil {
		return nil, err
	}

	meta, err := parseJSONFile[JSONManifest](metaPath)
	if err != nil {
		return nil, err
	}

	return meta, nil
}
