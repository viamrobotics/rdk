// Package mygizmosummer implements an acme:component:gizmo and depends on another custom API.
package mygizmosummer

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// Model is the full model definition.
var Model = resource.NewModel("acme", "demo", "mygizmosummer")

// Config is the gizmo model's config.
type Config struct {
	Summer string `json:"Summer"`
}

// Validate ensures that `Summer` is a non-empty string.
// Validation error will stop the associated resource from building.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.Summer == "" {
		return nil, fmt.Errorf(`expected "summer" attribute for myGizmo %q`, path)
	}

	// there are no dependencies for this model, so we return an empty list of strings
	return []string{cfg.Summer}, nil
}

func init() {
	resource.RegisterComponent(gizmoapi.API, Model, resource.Registration[gizmoapi.Gizmo, *Config]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (gizmoapi.Gizmo, error) {
			return NewMyGizmoSummer(deps, conf, logger)
		},
	})
}

type myActualGizmo struct {
	resource.Named
	resource.TriviallyCloseable

	mySummerMu sync.Mutex
	mySummer   summationapi.Summation
	logger     logging.Logger
}

// NewMyGizmoSummer returns a new mygizmosummer.
func NewMyGizmoSummer(
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (gizmoapi.Gizmo, error) {
	g := &myActualGizmo{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
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
	summer, err := resource.FromDependencies[summationapi.Summation](deps, summationapi.Named(gizmoConfig.Summer))
	if err != nil {
		return err
	}

	g.mySummerMu.Lock()
	g.mySummer = summer
	g.mySummerMu.Unlock()
	return nil
}

func (g *myActualGizmo) DoOne(ctx context.Context, arg1 string) (bool, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()

	n, err := strconv.ParseFloat(arg1, 64)
	if err != nil {
		return false, err
	}
	sum, err := g.mySummer.Sum(ctx, []float64{n})
	if err != nil {
		return false, err
	}
	return sum == n, nil
}

func (g *myActualGizmo) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()
	if len(arg1) == 0 {
		return false, nil
	}
	ns := []float64{}
	for _, arg := range arg1 {
		n, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return false, err
		}
		ns = append(ns, n)
	}
	sum, err := g.mySummer.Sum(ctx, ns)
	if err != nil {
		return false, err
	}
	return sum == 5, nil
}

func (g *myActualGizmo) DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()
	n, err := strconv.ParseFloat(arg1, 64)
	if err != nil {
		return nil, err
	}
	sum, err := g.mySummer.Sum(ctx, []float64{n})
	if err != nil {
		return nil, err
	}
	return []bool{sum == n, false, true, false}, nil
}

func (g *myActualGizmo) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()
	var rets []bool
	g.logger.Info(arg1)
	for _, arg := range arg1 {
		g.logger.Info(arg)
		n, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return nil, err
		}
		sum, err := g.mySummer.Sum(ctx, []float64{n})
		if err != nil {
			return nil, err
		}
		rets = append(rets, sum == n)
	}
	g.logger.Info(rets)
	return rets, nil
}

func (g *myActualGizmo) DoTwo(ctx context.Context, arg1 bool) (string, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()
	n := 1.0
	if !arg1 {
		n = 2.0
	}
	sum, err := g.mySummer.Sum(ctx, []float64{n, 3.0})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sum=%v", sum), nil
}

func (g *myActualGizmo) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
