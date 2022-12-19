package config

import (
	"os"
	"regexp"

	"github.com/pkg/errors"
)

var moduleNameRegEx = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

const reservedModuleName = "parent"

// Module represents an external resource module, with a path to the binary module file.
type Module struct {
	// Name is an arbitrary name used to identify the module, and is used to name it's socket as well.
	Name string `json:"name"`
	// ExePath is the path (either absolute, or relative to the working directory) to the executable module file.
	ExePath string `json:"executable_path"`
}

// Validate checks if the config is valid.
func (m *Module) Validate(path string) error {
	_, err := os.Stat(m.ExePath)
	if err != nil {
		return errors.Wrapf(err, "module %s executable path error", path)
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
