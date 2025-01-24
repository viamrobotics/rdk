// Package fake implements fake switches with different position counts.
package fake

import (
	"context"
	"fmt"
	"sync"

	switch_component "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var (
	model2Way  = resource.DefaultModelFamily.WithModel("fake-2way")
	model3Way  = resource.DefaultModelFamily.WithModel("fake-3way")
	model10Way = resource.DefaultModelFamily.WithModel("fake-10way")
)

// Config is the config for a fake switch.
type Config struct {
	resource.TriviallyValidateConfig
}

func init() {
	// Register all three switch models
	resource.RegisterComponent(switch_component.API, model2Way, resource.Registration[switch_component.Switch, *Config]{
		Constructor: func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (switch_component.Switch, error) {
			return NewSwitch(ctx, deps, conf, logger, 2)
		},
	})
	resource.RegisterComponent(switch_component.API, model3Way, resource.Registration[switch_component.Switch, *Config]{
		Constructor: func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (switch_component.Switch, error) {
			return NewSwitch(ctx, deps, conf, logger, 3)
		},
	})
	resource.RegisterComponent(switch_component.API, model10Way, resource.Registration[switch_component.Switch, *Config]{
		Constructor: func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (switch_component.Switch, error) {
			return NewSwitch(ctx, deps, conf, logger, 10)
		},
	})
}

// Switch is a fake switch that can be set to different positions.
type Switch struct {
	resource.Named
	resource.TriviallyCloseable
	mu            sync.Mutex
	logger        logging.Logger
	position      uint32
	positionCount int
}

// NewSwitch instantiates a new switch of the fake model type.
func NewSwitch(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
	positionCount int,
) (switch_component.Switch, error) {
	s := &Switch{
		Named:         conf.ResourceName().AsNamed(),
		logger:        logger,
		position:      0,
		positionCount: positionCount,
	}
	if err := s.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return s, nil
}

// Reconfigure reconfigures the switch atomically and in place.
func (s *Switch) Reconfigure(_ context.Context, _ resource.Dependencies, conf resource.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

// SetPosition sets the switch to the specified position.
func (s *Switch) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if position >= uint32(s.positionCount) {
		return fmt.Errorf("switch component %v position %d is invalid (valid range: 0-%d)", s.Name(), position, s.positionCount-1)
	}
	s.position = position
	return nil
}

// GetPosition returns the current position of the switch.
func (s *Switch) GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.position, nil
}

// GetNumberOfPositions returns the total number of valid positions for this switch.
func (s *Switch) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (int, error) {
	return s.positionCount, nil
}

// DoCommand executes a command on the switch.
func (s *Switch) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
