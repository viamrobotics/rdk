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
	// LogLevel represents the level at which the module should log its messages. It will be passed as a commandline
	// argument "log-level" (i.e. preceded by "--log-level=") to the module executable. If unset or set to an empty
	// string, "--log-level=debug" will be passed to the module executable if the server was started with "-debug".
	//
	// SDK logger-creation utilities, such as module.NewLoggerFromArgs, will create an "Info" level logger when any
	// value besides "" or "debug" is used for LogLevel ("log_level" in JSON). In other words, setting a LogLevel
	// of something like "info" will ignore the debug setting on the server.
	LogLevel string `json:"log_level"`
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
