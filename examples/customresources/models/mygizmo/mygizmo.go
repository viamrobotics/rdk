// Package mygizmo implements an acme:component:gizmo, a demonstration component that simply shows the various methods available in grpc.
package mygizmo

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/resource"
)

// Model is the full model definition.
var Model = resource.NewModel("acme", "demo", "mygizmo")

// Config is the gizmo model's config.
type Config struct {
	Arg1 string `json:"arg1"`
}

// Validate checks the attribute of myGizmo to ensure that an "arg1" attribute exists
// the model will not initialize if this is not set.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.Arg1 == "" {
		return nil, fmt.Errorf(`expected "Arg1" attribute for myGizmo %q`, path)
	}

	// there are no dependencies for this model, so we return an empty list of strings
	return []string{}, nil
}

func init() {
	resource.RegisterComponent(gizmoapi.API, Model, resource.Registration[gizmoapi.Gizmo, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (gizmoapi.Gizmo, error) {
			return NewMyGizmo(deps, conf, logger)
		},
	})
}

type myActualGizmo struct {
	resource.Named
	resource.TriviallyCloseable

	myArgMu sync.Mutex
	myArg   string
}

// NewMyGizmo returns a new mygizmo.
func NewMyGizmo(
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (gizmoapi.Gizmo, error) {
	g := &myActualGizmo{
		Named: conf.ResourceName().AsNamed(),
	}
	if err := g.Reconfigure(context.Background(), deps, conf); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *myActualGizmo) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	// This takes the generic resource.Config passed down from the parent and converts it to the
	// model-specific (aka "native") Config structure defined above making it easier to directly access attributes.
	gizmoConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	g.myArgMu.Lock()
	g.myArg = gizmoConfig.Arg1
	g.myArgMu.Unlock()
	return nil
}

func (g *myActualGizmo) DoOne(ctx context.Context, arg1 string) (bool, error) {
	g.myArgMu.Lock()
	defer g.myArgMu.Unlock()
	return arg1 == g.myArg, nil
}

func (g *myActualGizmo) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	g.myArgMu.Lock()
	defer g.myArgMu.Unlock()
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
	g.myArgMu.Lock()
	defer g.myArgMu.Unlock()
	return []bool{arg1 == g.myArg, false, true, false}, nil
}

func (g *myActualGizmo) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	g.myArgMu.Lock()
	defer g.myArgMu.Unlock()
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
