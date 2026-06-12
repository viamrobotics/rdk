// Package mygizmosummer implements an acme:component:gizmo and depends on another custom API.
package mygizmosummer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils/contextutils"
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
		return nil, nil, fmt.Errorf(`expected "summer" attribute for myGizmo %q`, path)
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
	summer, err := resource.FromProvider[summationapi.Summation](deps, summationapi.Named(gizmoConfig.Summer))
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

	if incoming, ok := contextutils.Metadata(ctx); ok {
		for k, vals := range incoming {
			if k == "arbitrary-md-from-client" && len(vals) == 2 {
				// test merge
				ctx = contextutils.AppendMetadata(ctx, "arbitrary-md-from-client", "arbitrary-md-from-client-val2")
				ctx = contextutils.AppendMetadata(ctx, "arbitrary-md-from-client", "arbitrary-md-from-client-val3-from-middle")
			}
		}
		ctx = contextutils.AppendMetadata(ctx, "arbitrary-md-from-middle", "arbitrary-md-from-middle-val1")
	}

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

func allExpectedMetadataPresentTestHelper(md contextutils.ViamMD) bool {
	numGood := 0
	for k, vals := range md {
		switch {
		case k == "arbitrary-md-from-client" && len(vals) == 2 &&
			slices.Contains(vals, "arbitrary-md-from-client-val1"):
			numGood++
		case k == "arbitrary-md-local-func-modify" && len(vals) == 2 && vals[0] == "real" && vals[1] == "real":
			numGood++
		case k == "opid" && len(vals) == 1 && slices.Contains(vals, "custom"):
			numGood++
		default:
			numGood--
		}
	}
	return numGood == 3
}

func (g *myActualGizmo) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	g.mySummerMu.Lock()
	defer g.mySummerMu.Unlock()

	if incoming, ok := contextutils.Metadata(ctx); ok {
		if allExpectedMetadataPresentTestHelper(incoming) {
			return false, errors.New("TestMetadataAcrossTwoModules-ClientStream-good")
		}
	}

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

	if incoming, ok := contextutils.Metadata(ctx); ok {
		if allExpectedMetadataPresentTestHelper(incoming) {
			return []bool{false}, errors.New("TestMetadataAcrossTwoModules-ServerStream-good")
		}
	}

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

	if incoming, ok := contextutils.Metadata(ctx); ok {
		if allExpectedMetadataPresentTestHelper(incoming) {
			return []bool{false}, errors.New("TestMetadataAcrossTwoModules-BiDiStream-good")
		}
	}

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
