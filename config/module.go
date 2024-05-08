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

// SyntheticPackage creates a fake package for a local module which points to a local tarball.
func (m Module) SyntheticPackage() (PackageConfig, error) {
	var ret PackageConfig
	if m.Type != ModuleTypeLocal {
		return ret, errors.New("non-local package passed to syntheticPackage")
	}
	ret.Name = fmt.Sprintf("synthetic-%s", m.Name)
	ret.Package = ret.Name
	ret.Type = PackageTypeModule
	ret.LocalPath = m.ExePath
	return ret, nil
}

// syntheticPackageExeDir returns the unpacked ExePath for local tarball modules.
func (m Module) syntheticPackageExeDir() (string, error) {
	pkg, err := m.SyntheticPackage()
	if err != nil {
		return "", err
	}
	return pkg.LocalDataDirectory(viamPackagesDir), nil
}

// EntrypointOnlyMetaJSON is a miniature version . We do this to avoid a circular dep between CLI and RDK.
// Better option is to move meta.json definition to this config package.
type EntrypointOnlyMetaJSON struct {
	Entrypoint string `json:"entrypoint"`
}

// EvaluateExePath returns absolute ExePath except for local tarballs where it looks for side-by-side meta.json.
func (m Module) EvaluateExePath() (string, error) {
	if m.NeedsSyntheticPackage() {
		metaPath := filepath.Join(filepath.Dir(m.ExePath), "meta.json")
		raw, err := os.ReadFile(metaPath) //nolint:gosec
		if err != nil {
			return "", errors.Wrap(err, "loading meta.json for local tarball")
		}
		var meta EntrypointOnlyMetaJSON
		err = json.Unmarshal(raw, &meta)
		if err != nil {
			return "", errors.Wrap(err, "parsing meta.json for local tarball")
		}
		exeDir, err := m.syntheticPackageExeDir()
		if err != nil {
			return "", err
		}
		return filepath.Abs(filepath.Join(exeDir, meta.Entrypoint))
	}
	return filepath.Abs(m.ExePath)
}
