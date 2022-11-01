package config

import (
	"os"
	"regexp"

	"github.com/pkg/errors"
)

// Module represents an external resource module, with path to a binary.
type Module struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Validate checks if the config is valid.
func (m *Module) Validate(path string) error {
	_, err := os.Stat(m.Path)
	if err != nil {
		return errors.Wrap(err, "module path error")
	}

	// the module name is used to create the socket path
	nameRegEx := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	if !nameRegEx.MatchString(m.Name) {
		return errors.New("module name must contain only letters, numbers, underscores and hyphens")
	}

	if m.Name == "parent" {
		return errors.New("cannot use module name of 'parent'")
	}

	return nil
}
