package config

import (
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

var moduleNameRegEx = regexp.MustCompile(`^[\w-]+$`)

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
	Status           *appValidationStatus `json:"status"`
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
	if m.Status != nil {
		return errors.New(m.Status.Error)
	}
	if m.alreadyValidated {
		return m.cachedErr
	}
	m.cachedErr = m.validate(path)
	m.alreadyValidated = true
	return m.cachedErr
}

func (m *Module) validate(path string) error {
	// Only check if the path exists during validation for local modules because the packagemanager may not have downloaded
	// the package yet.
	// As of 2023-08, modules can't know if they were originally registry modules, so this roundabout check is required
	if !(ContainsPlaceholder(m.ExePath) || strings.HasPrefix(m.ExePath, viamPackagesDir)) {
		_, err := os.Stat(m.ExePath)
		if err != nil {
			return errors.Wrapf(err, "module %s executable path error", path)
		}
	}

	// the module name is used to create the socket path
	if !moduleNameRegEx.MatchString(m.Name) {
		return errors.Errorf("module %s name must contain only letters, numbers, underscores and hyphens", path)
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
