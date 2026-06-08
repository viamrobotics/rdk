// Package mysum implements an acme:service:summation, a demo service which sums (or subtracts) a given list of numbers.
package mysum

import (
	"context"
	"errors"
	"slices"
	"sync"

	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils/contextutils"
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

	arbitraryMDFromServer := metadata.MD{}
	foundKeys := 0
	expectedKeys := 4
	if incoming, ok := contextutils.Metadata(ctx); ok {
		for k, vals := range incoming {
			switch {
			case k == "arbitrary-md-from-client" &&
				len(vals) == 4 && // middle module adds val2 again, so there will be 2
				slices.Contains(vals, "arbitrary-md-from-client-val1") &&
				slices.Contains(vals, "arbitrary-md-from-client-val2") &&
				slices.Contains(vals, "arbitrary-md-from-client-val3-from-middle"):
				arbitraryMDFromServer["from_client_md_good"] = []string{"true"}
			case k == "arbitrary-md-from-middle" &&
				len(vals) == 1 &&
				slices.Contains(vals, "arbitrary-md-from-middle-val1"):
				arbitraryMDFromServer["from_middle_md_good"] = []string{"true"}
			case k == "opid":
				// real opid is still present in metadata.FromIncomingContext
				if len(vals) == 1 && slices.Contains(vals, "custom") {
					arbitraryMDFromServer["custom_opid_good"] = []string{"true"}
				}
			case k == "arbitrary-md-local-func-modify":
				if len(vals) == 2 && vals[0] == "real" && vals[1] == "real" {
					arbitraryMDFromServer["from_client_md_modified_good"] = []string{"true"}
				}
			}
			foundKeys++
		}
	}
	if foundKeys != expectedKeys {
		arbitraryMDFromServer["unknown_metadata_found"] = []string{"true"}
	}
	arbitraryMDFromServer["arbitrary-md-to-client-from-end"] = []string{"arbitrary-md-to-client-from-end-val1"}
	arbitraryMDFromServer["arbitrary-md-to-client-from-end2"] = []string{"arbitrary-md-to-client-from-end2-val1"}
	//nolint:errcheck
	_ = contextutils.SetHeader(ctx, arbitraryMDFromServer)

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
