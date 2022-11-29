// Package mysum implements an acme:service:summation, a demo service which sums (or subtracts) a given list of numbers.
package mysum

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel(
	resource.Namespace("acme"),
	resource.ModelFamilyName("demo"),
	resource.ModelName("mysum"),
)

func init() {
	registry.RegisterService(summationapi.Subtype, Model, registry.Service{
		Constructor: newMySum,
	})
}

type mySum struct {
	mu sync.Mutex
	subtract bool
}

func newMySum(ctx context.Context, deps registry.Dependencies, cfg config.Service, logger *zap.SugaredLogger) (interface{}, error) {
	return &mySum{subtract: cfg.Attributes.Bool("subtract", false)}, nil
}

func (m *mySum) Sum(ctx context.Context, nums []float64) (float64, error) {
	if len(nums) <= 0 {
		return 0, errors.New("must provide at least one number to sum")
	}
	var ret float64
	for _, n := range nums {
		if m.subtract {
			ret -= n
		}else{
			ret += n
		}
	}
	return ret, nil
}

func (m *mySum)	Reconfigure(ctx context.Context, cfg config.Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subtract = cfg.Attributes.Bool("subtract", false)
	return nil
}
