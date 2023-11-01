// Package mysum implements an acme:service:summation, a demo service which sums (or subtracts) a given list of numbers.
package mysum

import (
	"context"
	"errors"
	"sync"

	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// Model is the full model definition.
var Model = resource.NewModel("acme", "demo", "mysum")

// Config is the sum model's config.
type Config struct {
	Subtract bool `json:"subtract,omitempty"` // the omitempty defaults the bool to golang's default of false

	// Embed TriviallyValidateConfig to make config validation a no-op. We will not check if any attributes exist
	// or are set to anything in particular, and there will be no implicit dependencies.
	// Config structs used in resource registration must implement Validate.
	resource.TriviallyValidateConfig
}

func init() {
	resource.RegisterService(summationapi.API, Model, resource.Registration[summationapi.Summation, *Config]{
		Constructor: newMySum,
	})
}

type mySum struct {
	resource.Named
	resource.TriviallyCloseable

	mu       sync.Mutex
	subtract bool
}

func newMySum(ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (summationapi.Summation, error) {
	summer := &mySum{
		Named: conf.ResourceName().AsNamed(),
	}
	if err := summer.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return summer, nil
}

func (m *mySum) Sum(ctx context.Context, nums []float64) (float64, error) {
	if len(nums) == 0 {
		return 0, errors.New("must provide at least one number to sum")
	}
	var ret float64
	for _, n := range nums {
		if m.subtract {
			ret -= n
		} else {
			ret += n
		}
	}
	return ret, nil
}

func (m *mySum) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	// This takes the generic resource.Config passed down from the parent and converts it to the
	// model-specific (aka "native") Config structure defined above making it easier to directly access attributes.
	sumConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.subtract = sumConfig.Subtract
	return nil
}
