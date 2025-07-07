// Package fake implements a fake button.
package fake

import (
	"context"
	"time"

	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("fake")

func init() {
	resource.RegisterComponent(button.API, model, resource.Registration[button.Button, *resource.NoNativeConfig]{Constructor: NewButton})
}

// Button is a fake button that logs when it is pressed.
type Button struct {
	resource.Named
	resource.TriviallyCloseable
	resource.AlwaysRebuild
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
	return b, nil
}

// Push logs the push.
func (b *Button) Push(ctx context.Context, extra map[string]interface{}) error {
	b.logger.Infof("Button pushed at %s", time.Now().Format(time.RFC3339))
	return nil
}
