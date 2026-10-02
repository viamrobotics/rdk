package obstacle

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{XMm: 100, YMm: 200, ZMm: 300}
	_, _, err := valid.Validate("")
	test.That(t, err, test.ShouldBeNil)

	for _, tc := range []struct {
		name        string
		cfg         Config
		errContains string
	}{
		{"missing x", Config{YMm: 1, ZMm: 1}, `Field: "x_mm"`},
		{"missing y", Config{XMm: 1, ZMm: 1}, `Field: "y_mm"`},
		{"missing z", Config{XMm: 1, YMm: 1}, `Field: "z_mm"`},
		{"negative dimension", Config{XMm: -1, YMm: 1, ZMm: 1}, "x_mm must be a positive number"},
		{"nan dimension", Config{XMm: 1, YMm: math.NaN(), ZMm: 1}, "y_mm must be a positive number"},
		{"inf dimension", Config{XMm: 1, YMm: 1, ZMm: math.Inf(1)}, "z_mm must be a positive number"},
		{"negative thickness", Config{XMm: 1, YMm: 1, ZMm: 1, WallThicknessMm: -5}, "wall_thickness_mm must be a positive number"},
		{"unknown face", Config{XMm: 1, YMm: 1, ZMm: 1, Exclude: []string{"roof"}}, `unknown face "roof"`},
		{"duplicate face", Config{XMm: 1, YMm: 1, ZMm: 1, Exclude: []string{LabelFloor, LabelFloor}}, "more than once"},
		{"all faces excluded", Config{XMm: 1, YMm: 1, ZMm: 1, Exclude: FaceLabels}, "leaves no faces"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.cfg.Validate("path")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errContains)
		})
	}

	defaults := valid.withDefaults()
	test.That(t, defaults.WallThicknessMm, test.ShouldEqual, defaultWallThicknessMM)
	explicit := Config{XMm: 1, YMm: 1, ZMm: 1, WallThicknessMm: 3}.withDefaults()
	test.That(t, explicit.WallThicknessMm, test.ShouldEqual, 3)
}

type boxSpec struct {
	center r3.Vector
	dims   r3.Vector
}

func boxSpecOf(t *testing.T, g spatialmath.Geometry) boxSpec {
	t.Helper()
	cfg, err := spatialmath.NewGeometryConfig(g)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfg.Type, test.ShouldEqual, spatialmath.BoxType)
	return boxSpec{center: g.Pose().Point(), dims: r3.Vector{X: cfg.X, Y: cfg.Y, Z: cfg.Z}}
}

func boxSpecsOf(t *testing.T, geometries []spatialmath.Geometry) map[string]boxSpec {
	t.Helper()
	specs := make(map[string]boxSpec, len(geometries))
	for _, g := range geometries {
		specs[g.Label()] = boxSpecOf(t, g)
	}
	return specs
}

func TestWallGeometries(t *testing.T) {
	cfg := Config{XMm: 400, YMm: 600, ZMm: 800, WallThicknessMm: 10}

	geometries, err := WallGeometries(cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometries, test.ShouldHaveLength, 6)
	test.That(t, boxSpecsOf(t, geometries), test.ShouldResemble, map[string]boxSpec{
		LabelXMax:    {r3.Vector{X: 205, Y: 0, Z: 0}, r3.Vector{X: 10, Y: 620, Z: 820}},
		LabelXMin:    {r3.Vector{X: -205, Y: 0, Z: 0}, r3.Vector{X: 10, Y: 620, Z: 820}},
		LabelYMax:    {r3.Vector{X: 0, Y: 305, Z: 0}, r3.Vector{X: 420, Y: 10, Z: 820}},
		LabelYMin:    {r3.Vector{X: 0, Y: -305, Z: 0}, r3.Vector{X: 420, Y: 10, Z: 820}},
		LabelFloor:   {r3.Vector{X: 0, Y: 0, Z: -405}, r3.Vector{X: 420, Y: 620, Z: 10}},
		LabelCeiling: {r3.Vector{X: 0, Y: 0, Z: 405}, r3.Vector{X: 420, Y: 620, Z: 10}},
	})

	cfg.Exclude = []string{LabelFloor, LabelXMin}
	geometries, err = WallGeometries(cfg)
	test.That(t, err, test.ShouldBeNil)
	specs := boxSpecsOf(t, geometries)
	test.That(t, specs, test.ShouldHaveLength, 4)
	_, hasFloor := specs[LabelFloor]
	test.That(t, hasFloor, test.ShouldBeFalse)
	_, hasXMin := specs[LabelXMin]
	test.That(t, hasXMin, test.ShouldBeFalse)
}

func newTestBounds(t *testing.T, cfg *Config, frame *referenceframe.LinkConfig) *bounds {
	t.Helper()
	res, err := newBounds(context.Background(), nil, resource.Config{
		Name:                "bounds",
		API:                 generic.API,
		Model:               BoundsModel,
		Frame:               frame,
		ConvertedAttributes: cfg,
	}, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	return res.(*bounds)
}

func TestNewBounds(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{XMm: 100, YMm: 100, ZMm: 100}
	b := newTestBounds(t, cfg, &referenceframe.LinkConfig{Parent: referenceframe.World})
	test.That(t, b.Name(), test.ShouldResemble, generic.Named("bounds"))

	geometries, err := b.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometries, test.ShouldHaveLength, 6)

	// Callers get copies.
	geometries[0].SetLabel("mutated")
	again, err := b.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, again[0].Label(), test.ShouldEqual, LabelXMax)

	// A frame geometry or a missing frame only warns.
	newTestBounds(t, cfg, &referenceframe.LinkConfig{
		Parent:   referenceframe.World,
		Geometry: &spatialmath.GeometryConfig{Type: spatialmath.BoxType, X: 1, Y: 1, Z: 1},
	})
	newTestBounds(t, cfg, nil)
}

func TestDoCommand(t *testing.T) {
	ctx := context.Background()
	b := newTestBounds(t, &Config{XMm: 100, YMm: 100, ZMm: 100}, &referenceframe.LinkConfig{Parent: referenceframe.World})

	resp, err := b.DoCommand(ctx, map[string]interface{}{CommandKey: CommandGet})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{
		"x_mm":              100.,
		"y_mm":              100.,
		"z_mm":              100.,
		"wall_thickness_mm": defaultWallThicknessMM,
		"exclude":           []string(nil),
		"geometry_count":    6,
	})

	// Lists arrive as []interface{} when the command came over the wire.
	resp, err = b.DoCommand(ctx, map[string]interface{}{
		CommandKey: CommandSet,
		"x_mm":     500.,
		"exclude":  []interface{}{LabelFloor},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["x_mm"], test.ShouldEqual, 500.)
	test.That(t, resp["y_mm"], test.ShouldEqual, 100.)
	test.That(t, resp["exclude"], test.ShouldResemble, []string{LabelFloor})
	test.That(t, resp["geometry_count"], test.ShouldEqual, 5)

	geometries, err := b.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	specs := boxSpecsOf(t, geometries)
	test.That(t, specs, test.ShouldHaveLength, 5)
	_, hasFloor := specs[LabelFloor]
	test.That(t, hasFloor, test.ShouldBeFalse)
	test.That(t, specs[LabelXMax].center, test.ShouldResemble, r3.Vector{X: 255, Y: 0, Z: 0})

	// Invalid updates are rejected and leave the enclosure untouched.
	for _, cmd := range []map[string]interface{}{
		{CommandKey: CommandSet, "x_mm": -1.},
		{CommandKey: CommandSet, "x_mm": "wide"},
		{CommandKey: CommandSet, "exclude": "floor"},
		{CommandKey: CommandSet, "exclude": []interface{}{7.}},
		{CommandKey: CommandSet, "exclude": []interface{}{"roof"}},
		{CommandKey: CommandSet, "exclude": FaceLabels},
		{CommandKey: "bogus"},
		{},
	} {
		_, err = b.DoCommand(ctx, cmd)
		test.That(t, err, test.ShouldNotBeNil)
	}
	resp, err = b.DoCommand(ctx, map[string]interface{}{CommandKey: CommandGet})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["x_mm"], test.ShouldEqual, 500.)
	test.That(t, resp["geometry_count"], test.ShouldEqual, 5)
}
