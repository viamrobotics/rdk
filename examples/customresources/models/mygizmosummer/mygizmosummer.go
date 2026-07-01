// Package mygizmosummer implements an acme:component:gizmo and depends on another custom API.
package mygizmosummer

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"sync"

	"braces.dev/errtrace"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils/contextutils/metadata"
)

// Model is the full model definition.
var Model = resource.NewModel("acme", "demo", "mygizmosummer")

// Config is the gizmo model's config.
type Config struct {
	Summer string `json:"Summer"`
}

// Validate ensures that `Summer` is a non-empty string.
// Validation error will stop the associated resource from building.
func (cfg *Config) Validate(path string) ([]string, []string, error) {
	if cfg.Summer == "" {
		return nil, nil, errtrace.Wrap(fmt.Errorf(`expected "summer" attribute for myGizmo %q`, path))
	}

	// there are no dependencies for this model, so we return an empty list of strings
	return []string{cfg.Summer}, nil, nil
}

func init() {
	resource.RegisterComponent(gizmoapi.API, Model, resource.Registration[gizmoapi.Gizmo, *Config]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (gizmoapi.Gizmo, error) {
			return errtrace.Wrap2(NewMyGizmoSummer(deps, conf, logger))
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
	if err := g.reconfigure(context.Background(), deps, conf); err != nil {
		return nil, errtrace.Wrap(err)
	}
	return g, nil
}

func (g *myActualGizmo) reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	// This takes the generic resource.Config passed down from the parent and converts it to the
	// model-specific (aka "native") Config structure defined above making it easier to directly access attributes.
	gizmoConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return errtrace.Wrap(err)
	}
	summer, err := resource.FromProvider[summationapi.Summation](deps, summationapi.Named(gizmoConfig.Summer))
	if err != nil {
		return errtrace.Wrap(err)
	}

	g.mySummerMu.Lock()
	g.mySummer = summer
	g.mySummerMu.Unlock()
	return nil
}

func (g *myActualGizmo) DoOne(ctx context.Context, arg1 string) (bool, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()

	for k, v := range metadata.All(ctx) {
		if k == "arbitrary-md-from-client2" && v == "arbitrary-md-from-client-val2" {
			// test replacing one field
			ctx = metadata.Set(ctx, "arbitrary-md-from-client2", "arbitrary-md-from-client-val3-from-middle")
		}
		ctx = metadata.Set(ctx, "arbitrary-md-from-middle", "arbitrary-md-from-middle-val1")
	}

	n, err := strconv.ParseFloat(arg1, 64)
	if err != nil {
		return false, errtrace.Wrap(err)
	}

	var sum float64
	sum, err = g.mySummer.Sum(ctx, []float64{n})
	if err != nil {
		return false, errtrace.Wrap(err)
	}
	return sum == n, nil
}

func allExpectedMetadataPresentTestHelper(md metadata.ViamMD) bool {
	numGood := 0
	for k, v := range md {
		switch {
		case k == "arbitrary-md-from-client" && v == "arbitrary-md-from-client-val1":
			numGood++
		case k == "arbitrary-md-from-client2" && v == "arbitrary-md-from-client-val2":
			numGood++
		case k == "arbitrary-md-local-func-modify" && v == "real":
			numGood++
		case k == "opid" && v == "custom":
			numGood++
		default:
			numGood--
		}
	}
	return numGood == 4
}

func (g *myActualGizmo) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()

	if allExpectedMetadataPresentTestHelper(maps.Collect(metadata.All(ctx))) {
		return false, errtrace.Wrap(errors.New("TestMetadataAcrossTwoModules-ClientStream-good"))
	}

	if len(arg1) == 0 {
		return false, nil
	}
	ns := []float64{}
	for _, arg := range arg1 {
		n, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return false, errtrace.Wrap(err)
		}
		ns = append(ns, n)
	}
	sum, err := g.mySummer.Sum(ctx, ns)
	if err != nil {
		return false, errtrace.Wrap(err)
	}
	return sum == 5, nil
}

func (g *myActualGizmo) DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()

	if allExpectedMetadataPresentTestHelper(maps.Collect(metadata.All(ctx))) {
		return []bool{false}, errtrace.Wrap(errors.New("TestMetadataAcrossTwoModules-ServerStream-good"))
	}

	n, err := strconv.ParseFloat(arg1, 64)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	sum, err := g.mySummer.Sum(ctx, []float64{n})
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return []bool{sum == n, false, true, false}, nil
}

func (g *myActualGizmo) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()

	if allExpectedMetadataPresentTestHelper(maps.Collect(metadata.All(ctx))) {
		return []bool{false}, errtrace.Wrap(errors.New("TestMetadataAcrossTwoModules-BiDiStream-good"))
	}

	var rets []bool
	g.logger.Info(arg1)
	for _, arg := range arg1 {
		g.logger.Info(arg)
		n, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return nil, errtrace.Wrap(err)
		}
		sum, err := g.mySummer.Sum(ctx, []float64{n})
		if err != nil {
			return nil, errtrace.Wrap(err)
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
		return "", errtrace.Wrap(err)
	}
	return fmt.Sprintf("sum=%v", sum), nil
}

func (g *myActualGizmo) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
