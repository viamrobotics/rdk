// Package pexec defines process management utilities to be used as a library within
// a go process wishing to own sub-processes.
//
// It helps manage the lifecycle of processes by keeping them up as long as possible
// when configured.
package pexec

import (
	"encoding/json"
	"io"
	"reflect"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/utils"
)

// defaultStopTimeout is how long to wait in seconds (all stages) between first signaling and finally killing.
const defaultStopTimeout = time.Second * 10

// A ProcessConfig describes how to manage a system process.
type ProcessConfig struct {
	ID      string
	Name    string
	Args    []string
	CWD     string
	OneShot bool
	// Optional. When present, we will try to look up the Uid of the named user
	// and run the process as that user.
	Username string
	// Environment variables to pass through to the process.
	// Will overwrite existing environment variables.
	Environment map[string]string
	Log         bool
	LogWriter   io.Writer
	StopSignal  syscall.Signal
	StopTimeout time.Duration
	// OnUnexpectedExit will be called when the manage goroutine detects an
	// unexpected exit of the process. The exit code of the crashed process will
	// be passed in. If the returned bool is true, the manage goroutine will
	// attempt to restart the process. Otherwise, the manage goroutine will
	// simply return.
	//
	// NOTE(benjirewis): use `jsonschema:"-"` struct tag to avoid issues with
	// jsonschema reflection (go functions cannot be encoded to JSON).
	OnUnexpectedExit UnexpectedExitHandler `jsonschema:"-"`
	// The logger to use for STDOUT of this process. If not specified, will use
	// a sublogger of the `logger` parameter given to `NewManagedProcess`.
	StdOutLogger utils.ZapCompatibleLogger
	// The logger to use for STDERR of this process. If not specified, will use
	// a sublogger of the `logger` parameter given to `NewManagedProcess`.
	StdErrLogger utils.ZapCompatibleLogger

	alreadyValidated bool
	cachedErr        error
}

// Validate ensures all parts of the config are valid.
func (config *ProcessConfig) Validate(path string) error {
	if config.alreadyValidated {
		return config.cachedErr
	}
	config.cachedErr = config.validate(path)
	config.alreadyValidated = true
	return config.cachedErr
}

func (config *ProcessConfig) validate(path string) error {
	if config.ID == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "id")
	}
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	if config.StopTimeout < 100*time.Millisecond && config.StopTimeout != 0 {
		return utils.NewConfigValidationError(path, errors.New("stop_timeout should not be less than 100ms"))
	}
	return nil
}

// Note: keep this in sync with json-supported fields in ProcessConfig.
type configData struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Args        []string          `json:"args"`
	CWD         string            `json:"cwd"`
	OneShot     bool              `json:"one_shot"`
	Username    string            `json:"username"`
	Environment map[string]string `json:"env"`
	Log         bool              `json:"log"`
	StopSignal  string            `json:"stop_signal,omitempty"`
	StopTimeout string            `json:"stop_timeout,omitempty"`
}

// UnmarshalJSON parses incoming json.
func (config *ProcessConfig) UnmarshalJSON(data []byte) error {
	var temp configData
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	*config = ProcessConfig{
		ID:          temp.ID,
		Name:        temp.Name,
		Args:        temp.Args,
		CWD:         temp.CWD,
		OneShot:     temp.OneShot,
		Username:    temp.Username,
		Environment: temp.Environment,
		Log:         temp.Log,
		// OnUnexpectedExit cannot be specified in JSON.
	}

	if temp.StopTimeout != "" {
		dur, err := time.ParseDuration(temp.StopTimeout)
		if err != nil {
			return err
		}
		config.StopTimeout = dur
	}

	stopSig, err := parseSignal(temp.StopSignal, "stop_signal")
	if err != nil {
		return err
	}
	config.StopSignal = stopSig

	return nil
}

// MarshalJSON converts to json.
func (config ProcessConfig) MarshalJSON() ([]byte, error) {
	var stopSig string
	if config.StopSignal != 0 {
		stopSig = config.StopSignal.String()
	}
	temp := configData{
		ID:          config.ID,
		Name:        config.Name,
		Args:        config.Args,
		CWD:         config.CWD,
		OneShot:     config.OneShot,
		Username:    config.Username,
		Environment: config.Environment,
		Log:         config.Log,
		StopSignal:  stopSig,
		StopTimeout: config.StopTimeout.String(),
		// OnUnexpectedExit cannot be converted to JSON.
	}
	return json.Marshal(temp)
}

// Equals checks if the two configs are deeply equal to each other.
func (config ProcessConfig) Equals(other ProcessConfig) bool {
	config.alreadyValidated = false
	config.cachedErr = nil
	other.alreadyValidated = false
	other.cachedErr = nil
	//nolint:govet
	return reflect.DeepEqual(config, other)
}
