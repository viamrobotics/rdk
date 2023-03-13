// Package mygizmo implements an acme:component:gizmo, a demonstration component that simply shows the various methods available in grpc.
package mygizmo

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel(
	resource.Namespace("acme"),
	resource.ModelFamilyName("demo"),
	resource.ModelName("mygizmo"),
)

func init() {
	registry.RegisterComponent(gizmoapi.Subtype, Model, registry.Component{
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
	mu    sync.Mutex
	myArg string
	generic.Echo
}

func NewMyGizmo(
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) gizmoapi.Gizmo {
	return &myActualGizmo{myArg: config.Attributes.String("arg1")}
}

func (g *myActualGizmo) DoOne(ctx context.Context, arg1 string) (bool, error) {
	return arg1 == g.myArg, nil
}

func (g *myActualGizmo) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	if len(arg1) == 0 {
		return false, nil
	}
	ret := true
	for _, arg := range arg1 {
		ret = ret && arg == g.myArg
	}
	return ret, nil
}

func (g *myActualGizmo) DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error) {
	return []bool{arg1 == g.myArg, false, true, false}, nil
}

func (g *myActualGizmo) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	var rets []bool
	for _, arg := range arg1 {
		rets = append(rets, arg == g.myArg)
	}
	return rets, nil
}

func (g *myActualGizmo) DoTwo(ctx context.Context, arg1 bool) (string, error) {
	return fmt.Sprintf("arg1=%t", arg1), nil
}

func (g *myActualGizmo) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func (g *myActualGizmo) Reconfigure(ctx context.Context, cfg config.Component, deps registry.Dependencies) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.myArg = cfg.Attributes.String("arg1")
	return nil
}
