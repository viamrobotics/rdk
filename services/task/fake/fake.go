// Package fake implements a task that takes a configurable amount of time and
// returns a configurable outcome, for testing state machines and UIs.
package fake

import (
	"context"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/task"
)

// Model is the fake task model.
var Model = resource.DefaultModelFamily.WithModel("fake")

// Config configures a fake task.
type Config struct {
	// Duration is how long one execution takes, e.g. "500ms". Default 100ms.
	Duration string `json:"duration,omitempty"`
	// Steps is how many progress updates an execution reports. Default 4.
	Steps int `json:"steps,omitempty"`
	// Outcome is the outcome an execution returns. Default "done". An
	// execution's input may override it with an "outcome" key.
	Outcome string `json:"outcome,omitempty"`
	// Output is the output an execution returns. The execution's input is
	// echoed under the "input" key.
	Output map[string]interface{} `json:"output,omitempty"`
}

// Validate validates the config.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	if conf.Duration != "" {
		if _, err := time.ParseDuration(conf.Duration); err != nil {
			return nil, nil, resource.NewConfigValidationError(path, err)
		}
	}
	return nil, nil, nil
}

func init() {
	resource.RegisterService(task.API, Model, resource.Registration[task.Service, *Config]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
		) (task.Service, error) {
			cfg, err := resource.NativeConfig[*Config](conf)
			if err != nil {
				return nil, err
			}
			return New(conf.ResourceName(), cfg), nil
		},
	})
}

// New returns a fake task with the given name and config.
func New(name resource.Name, cfg *Config) task.Service {
	duration := 100 * time.Millisecond
	if d, err := time.ParseDuration(cfg.Duration); err == nil && cfg.Duration != "" {
		duration = d
	}
	steps := cfg.Steps
	if steps <= 0 {
		steps = 4
	}
	outcome := cfg.Outcome
	if outcome == "" {
		outcome = task.OutcomeDone
	}
	return task.NewFromFunc(name, func(ctx context.Context, input map[string]interface{}, report task.ProgressFunc) (task.Result, error) {
		if o, ok := input["outcome"].(string); ok && o != "" {
			outcome = o
		}
		for i := 0; i < steps; i++ {
			report(task.Progress{Message: "working", Fraction: float64(i) / float64(steps)})
			select {
			case <-ctx.Done():
				return task.Result{}, ctx.Err()
			case <-time.After(duration / time.Duration(steps)):
			}
		}
		output := map[string]interface{}{"input": input}
		for k, v := range cfg.Output {
			output[k] = v
		}
		return task.Result{Outcome: outcome, Output: output}, nil
	})
}
