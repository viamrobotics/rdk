// Package mysum implements an acme:service:summation, a demo service which sums (or subtracts) a given list of numbers.
package mysum

import (
	"context"
	"errors"
	"sync"

	"braces.dev/errtrace"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils/contextutils/metadata"
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
	if err := summer.reconfigure(ctx, deps, conf); err != nil {
		return nil, errtrace.Wrap(err)
	}
	return summer, nil
}

func (m *mySum) Sum(ctx context.Context, nums []float64) (float64, error) {
	if len(nums) == 0 {
		return 0, errtrace.Wrap(errors.New("must provide at least one number to sum"))
	}

	numGood := 0

	foundKeys := 0
	expectedKeys := 4
	for k, v := range metadata.All(ctx) {
		switch {
		case k == "arbitrary-md-from-client" && v == "arbitrary-md-from-client-val1":
			numGood++
		case k == "arbitrary-md-from-client2" && v == "arbitrary-md-from-client-val3-from-middle":
			numGood++
		case k == "arbitrary-md-from-middle" && v == "arbitrary-md-from-middle-val1":
			numGood++
		case k == "opid" && v == "custom":
			// real opid is still present in metadata.FromIncomingContext
			numGood++
		case k == "arbitrary-md-local-func-modify" && v == "real":
			numGood++
		}
		foundKeys++
	}
	if foundKeys == expectedKeys {
		numGood++
	}
	if numGood == 5 {
		// used for TestMetadataAcrossTwoModules test only. in other cases, numGood should be 0
		return -1, errtrace.Wrap(errors.New("TestMetadataAcrossTwoModules-good"))
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

func (m *mySum) reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	// This takes the generic resource.Config passed down from the parent and converts it to the
	// model-specific (aka "native") Config structure defined above making it easier to directly access attributes.
	sumConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return errtrace.Wrap(err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.subtract = sumConfig.Subtract
	return nil
}
