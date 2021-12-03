package motor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.viam.com/utils"

	"go.viam.com/core/config"
)

// A PID represent a PID controller
type PID interface {
	// Config will return the underlying configuration of the PID controller
	Config(ctx context.Context) (*PIDConfig, error)

	// Output returns the discrete step of of the controller, dt is the delte time between two subsequent call, setPoint is the desired value, measured is the actual value
	Output(ctx context.Context, dt time.Duration, setPoint float64, measured float64) (float64, bool)

	// UpdateConfig update one or more gains of the PID controller
	UpdateConfig(ctx context.Context, cfg PIDConfig) error

	// Reset reset the internal state of the PID controller
	Reset() error
}

// CreatePID  Create a PID controller based on the config
func CreatePID(cfg *PIDConfig) (PID, error) {
	if cfg.Type == "basic" {
		pid := &BasicPID{
			cfg: cfg,
			Ki:  cfg.Attributes.Float64("Ki", 0),
			Kp:  cfg.Attributes.Float64("Kp", 0),
			Kd:  cfg.Attributes.Float64("Kd", 0),
		}
		return pid, nil
	}
	return nil, fmt.Errorf("unsupported PID type %s", cfg.Type)
}

// BasicPID is the standard implementation of a PID controller
type BasicPID struct {
	cfg   *PIDConfig
	error float64
	Ki    float64
	Kd    float64
	Kp    float64
	int   float64
	sat   int
}

// Config will return the underlying configuration of the PID controller
func (p *BasicPID) Config(ctx context.Context) (*PIDConfig, error) {
	if p.cfg == nil {
		return nil, errors.New("no config set for underlying PID")
	}
	return p.cfg, nil
}

// Output returns the discrete step of the PID controller, dt is the delta time between two subsequent call, setPoint is the desired value, measured is the measured value. Returns false when the output is invalid (the integral is saturating) in this case continue to use the last valid value
func (p *BasicPID) Output(ctx context.Context, dt time.Duration, setPoint float64, measured float64) (float64, bool) {
	dtS := dt.Seconds()
	error := setPoint - measured
	if (p.sat > 0 && error > 0) || (p.sat < 0 && error < 0) {
		return 0, false
	}
	p.int += p.Ki * p.error * dtS
	if p.int > 100 {
		p.int = 100
		p.sat = 1
	} else if p.int < 0 {
		p.int = 0
		p.sat = -1
	} else {
		p.sat = 0
	}
	deriv := (error - p.error) / dtS
	output := p.Kp*error + p.int + p.Kd*deriv
	p.error = error
	if output > 100 {
		output = 100
	} else if output < 0 {
		output = 0
	}
	return output, true

}

// UpdateConfig update one or more gains of the PID controller
func (p *BasicPID) UpdateConfig(ctx context.Context, cfg PIDConfig) error {
	p.Kp = cfg.Attributes.Float64("Kp", p.Kp)
	p.cfg.Attributes["Kp"] = p.Kp
	p.Kd = cfg.Attributes.Float64("Kd", p.Kd)
	p.cfg.Attributes["Kd"] = p.Kd
	p.Ki = cfg.Attributes.Float64("Ki", p.Ki)
	p.cfg.Attributes["Ki"] = p.Ki
	return nil
}

// Reset reset the internal state of the PID controller
func (p *BasicPID) Reset() error {
	p.int = 0
	p.error = 0
	p.sat = 0
	return nil
}

//PIDConfig represent the configuration of the a PID controller
type PIDConfig struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Attributes config.AttributeMap `json:"attributes"`
}

//Validate validates the configuration
func (config *PIDConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	if config.Attributes == nil {
		return utils.NewConfigValidationFieldRequiredError(path, "attributes")
	}
	return nil
}
