package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

const reservedModuleName = "parent"

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
			return errors.Wrapf(err, "module %s executable path error", path)
		}
	}

	if err := utils.ValidateModuleName(m.Name); err != nil {
		return resource.NewConfigValidationError(path, err)
	}

	if m.Name == reservedModuleName {
		return errors.Errorf("module %s cannot use the reserved name of %s", path, reservedModuleName)
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
		return nil, errors.Wrap(err, "reading json file")
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
			return "", errors.Wrapf(err, "couldn't find meta.json inside tarball %s (or next to it)", m.ExePath)
		}
		entrypoint, err := utils.SafeJoinDir(exeDir, meta.Entrypoint)
		if err != nil {
			return "", err
		}
		return filepath.Abs(entrypoint)
	}
	return m.ExePath, nil
}
