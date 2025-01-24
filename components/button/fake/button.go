// Package fake implements a fake button.
package fake

import (
	"context"
	"sync"

	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("fake")

// Config is the config for a fake button.
type Config struct {
	resource.TriviallyValidateConfig
}

func init() {
	resource.RegisterComponent(button.API, model, resource.Registration[button.Button, *Config]{Constructor: NewButton})
}

// Button is a fake button that logs when it is pressed
type Button struct {
	resource.Named
	resource.TriviallyCloseable
	mu     sync.Mutex
	logger logging.Logger
}

// NewButton instantiates a new button of the fake model type.
func NewButton(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (button.Button, error) {
	b := &Button{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}
	if err := b.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return b, nil
}

// Reconfigure reconfigures the button atomically and in place.
func (b *Button) Reconfigure(_ context.Context, _ resource.Dependencies, conf resource.Config) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return nil
}

// Push logs the push
func (b *Button) Push(ctx context.Context, extra map[string]interface{}) error {
	b.logger.Info("pushed button")
	return nil
}
