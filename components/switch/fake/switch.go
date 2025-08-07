// Package fake implements fake switches with different position counts.
package fake

import (
	"context"
	"fmt"
	"sync"

	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("fake")

// Config is the config for a fake switch.
type Config struct {
	resource.TriviallyValidateConfig

	// PositionCount is the number of positions that the switch can be in.
	// If omitted, the switch will have two positions.
	PositionCount *uint32 `json:"position_count"`

	// Labels is an array of labels corresponding to the positions.
	// If omitted, or if the length of the array
	// does not match PositionCount, the switch will not have labels.
	Labels []string `json:"labels"`
}

func init() {
	// Register all three switch models
	resource.RegisterComponent(toggleswitch.API, model, resource.Registration[toggleswitch.Switch, *Config]{
		Constructor: NewSwitch,
	})
}

// Switch is a fake switch that can be set to different positions.
type Switch struct {
	resource.Named
	resource.TriviallyCloseable
	resource.AlwaysRebuild
	mu            sync.Mutex
	logger        logging.Logger
	position      uint32
	positionCount uint32
	labels        []string
}

// NewSwitch instantiates a new switch of the fake model type.
func NewSwitch(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (toggleswitch.Switch, error) {
	s := &Switch{
		Named:         conf.ResourceName().AsNamed(),
		logger:        logger,
		position:      0,
		positionCount: 2,
		labels:        nil,
	}

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	if newConf.PositionCount != nil {
		s.positionCount = *newConf.PositionCount
	}

	if newConf.Labels != nil {
		s.labels = newConf.Labels
	}
	if len(s.labels) != int(s.positionCount) {
		s.labels = nil
	}

	return s, nil
}

// SetPosition sets the switch to the specified position.
func (s *Switch) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if position >= s.positionCount {
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
func (s *Switch) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
	return s.positionCount, s.labels, nil
}
