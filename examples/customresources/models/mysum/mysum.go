// Package mysum implements an acme:service:summation, a demo service which sums (or subtracts) a given list of numbers.
package mysum

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"

	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel("acme", "demo", "mysum")

func init() {
	resource.RegisterService(summationapi.API, Model, resource.Registration[summationapi.Summation, resource.NoNativeConfig]{
		Constructor: newMySum,
	})
}

type mySum struct {
	resource.Named
	resource.TriviallyCloseable

	mu       sync.Mutex
	subtract bool
}

func newMySum(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger *zap.SugaredLogger) (summationapi.Summation, error) {
	summer := &mySum{
		Named: conf.ResourceName().AsNamed(),
	}
	if err := summer.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return summer, nil
}

func (m *mySum) Sum(ctx context.Context, nums []float64) (float64, error) {
	if len(nums) <= 0 {
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subtract = conf.Attributes.Bool("subtract", false)
	return nil
}
