// Package mygizmo implements an acme:component:gizmo.
package mygizmo

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customcomponents/gizmoapi"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel(
	resource.Namespace("acme"),
	resource.ModelFamilyName("demo"),
	resource.ModelName("mygizmo"),
)

func init() {
	registry.RegisterComponent(gizmoapi.ResourceSubtype, Model, registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewMyGizmo(deps, config, logger), nil
		},
	})
}

type myActualGizmo struct {
	deps   registry.Dependencies
	config config.Component
	logger golog.Logger
}

func NewMyGizmo(
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) gizmoapi.Gizmo {
	return &myActualGizmo{deps, config, logger}
}

func (g *myActualGizmo) DoOne(ctx context.Context, arg1 string) (bool, error) {
	return arg1 == "arg1", nil
}

func (g *myActualGizmo) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	if len(arg1) == 0 {
		return false, nil
	}
	ret := true
	for _, arg := range arg1 {
		ret = ret && arg == "arg1"
	}
	return ret, nil
}

func (g *myActualGizmo) DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error) {
	return []bool{arg1 == "arg1", false, true, false}, nil
}

func (g *myActualGizmo) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	var rets []bool
	for _, arg := range arg1 {
		rets = append(rets, arg == "arg1")
	}
	return rets, nil
}

func (g *myActualGizmo) DoTwo(ctx context.Context, arg1 bool) (string, error) {
	return fmt.Sprintf("arg1=%t", arg1), nil
}
