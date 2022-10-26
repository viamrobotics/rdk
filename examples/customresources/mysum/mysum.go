// Package mysum implements an acme:service:summation.
package mysum

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/summationapi"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

var Model = resource.NewModel(
	resource.Namespace("acme"),
	resource.ModelFamilyName("demo"),
	resource.ModelName("mysum"),
)

func init() {
	registry.RegisterService(summationapi.ResourceSubtype, Model, registry.Service{
		Constructor: newMySum,
	})
}

type mySum struct {
	subtract bool
}

func newMySum(ctx context.Context, r robot.Robot, cfg config.Service, logger *zap.SugaredLogger) (interface{}, error) {
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
