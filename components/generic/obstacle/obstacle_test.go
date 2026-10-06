package obstacle

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

func boxConfig(label string, x, y, z float64, offset r3.Vector) spatialmath.GeometryConfig {
	return spatialmath.GeometryConfig{Type: spatialmath.BoxType, X: x, Y: y, Z: z, TranslationOffset: offset, Label: label}
}

// shelfGeometries is a box raised and offset from the component frame, a sphere beside it, and an
// unlabeled point that gets a positional name.
func shelfGeometries() []spatialmath.GeometryConfig {
	return []spatialmath.GeometryConfig{
		boxConfig("box", 10, 20, 30, r3.Vector{X: 100, Y: 0, Z: 550}),
		{Type: spatialmath.SphereType, R: 10, TranslationOffset: r3.Vector{X: 0, Y: 200, Z: 0}, Label: "post"},
		{Type: spatialmath.PointType, TranslationOffset: r3.Vector{X: 1, Y: 2, Z: 3}},
	}
}

func TestConfigValidate(t *testing.T) {
	_, _, err := (&Config{Geometries: shelfGeometries()}).Validate("")
	test.That(t, err, test.ShouldBeNil)

	with := func(geometries ...spatialmath.GeometryConfig) Config { return Config{Geometries: geometries} }
	for _, tc := range []struct {
		name        string
		cfg         Config
		errContains string
	}{
		{"no geometries", Config{}, `Field: "geometries"`},
		{"bad geometry", with(spatialmath.GeometryConfig{Type: "blob"}), "blob"},
		{"negative box", with(boxConfig("a", -1, 1, 1, r3.Vector{})), "geometries.0"},
		{"bad orientation", with(spatialmath.GeometryConfig{
			Type: spatialmath.SphereType, R: 1, OrientationOffset: spatialmath.OrientationConfig{Type: "spin"},
		}), "spin"},
		{"world label", with(spatialmath.GeometryConfig{Type: spatialmath.SphereType, R: 1, Label: "world"}), "reserved"},
		{"colon label", with(spatialmath.GeometryConfig{Type: spatialmath.SphereType, R: 1, Label: "a:b"}), "may not contain"},
		{"duplicate label", with(boxConfig("a", 1, 1, 1, r3.Vector{}), boxConfig("a", 1, 1, 1, r3.Vector{})), "more than once"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.cfg.Validate("path")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errContains)
		})
	}
}

func newTestObstacle(t *testing.T, cfg *Config, frame *referenceframe.LinkConfig) *obstacle {
	t.Helper()
	res, err := newObstacle(context.Background(), nil, resource.Config{
		Name:                "obstacle",
		API:                 generic.API,
		Model:               Model,
		Frame:               frame,
		ConvertedAttributes: cfg,
	}, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	return res.(*obstacle)
}

func centersOf(geometries []spatialmath.Geometry) map[string]r3.Vector {
	centers := make(map[string]r3.Vector, len(geometries))
	for _, g := range geometries {
		centers[g.Label()] = g.Pose().Point()
	}
	return centers
}

func TestGeometriesAndKinematics(t *testing.T) {
	ctx := context.Background()
	o := newTestObstacle(t, &Config{Geometries: shelfGeometries()}, &referenceframe.LinkConfig{Parent: referenceframe.World})
	test.That(t, o.Name(), test.ShouldResemble, generic.Named("obstacle"))

	expected := map[string]r3.Vector{
		"box":        {X: 100, Y: 0, Z: 550},
		"post":       {X: 0, Y: 200, Z: 0},
		"geometry_2": {X: 1, Y: 2, Z: 3},
	}
	geometries, err := o.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, centersOf(geometries), test.ShouldResemble, expected)

	// Callers get copies.
	geometries[0].SetLabel("mutated")
	again, err := o.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, centersOf(again), test.ShouldResemble, expected)

	model, err := o.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(model.DoF()), test.ShouldEqual, 0)
	test.That(t, model.Name(), test.ShouldEqual, "obstacle")
	gif, err := model.Geometries([]referenceframe.Input{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, centersOf(gif.Geometries()), test.ShouldResemble, map[string]r3.Vector{
		"obstacle:box":        {X: 100, Y: 0, Z: 550},
		"obstacle:post":       {X: 0, Y: 200, Z: 0},
		"obstacle:geometry_2": {X: 1, Y: 2, Z: 3},
	})
	test.That(t, model.ModelConfig().Links, test.ShouldHaveLength, 3)

	// A frame geometry or a missing frame only warns.
	newTestObstacle(t, &Config{Geometries: shelfGeometries()}, &referenceframe.LinkConfig{
		Parent:   referenceframe.World,
		Geometry: &spatialmath.GeometryConfig{Type: spatialmath.BoxType, X: 1, Y: 1, Z: 1},
	})
	newTestObstacle(t, &Config{Geometries: shelfGeometries()}, nil)
}

func TestDoCommand(t *testing.T) {
	ctx := context.Background()
	o := newTestObstacle(t, &Config{Geometries: shelfGeometries()}, &referenceframe.LinkConfig{Parent: referenceframe.World})

	resp, err := o.DoCommand(ctx, map[string]interface{}{CommandKey: CommandGet})
	test.That(t, err, test.ShouldBeNil)
	geometries, ok := resp[GeometriesKey].([]interface{})
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, geometries, test.ShouldHaveLength, 3)
	test.That(t, geometries[1].(map[string]interface{})["Label"], test.ShouldEqual, "post")

	// Replace the geometries with a wire-shaped list. Labels may be given in either case.
	resp, err = o.DoCommand(ctx, map[string]interface{}{
		CommandKey: CommandSet,
		GeometriesKey: []interface{}{
			map[string]interface{}{
				"type": "box", "x": 500., "y": 10., "z": 400.,
				"translation": map[string]interface{}{"x": 0., "y": -300., "z": 0.},
				"label":       "wall",
			},
		},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp[GeometriesKey].([]interface{}), test.ShouldHaveLength, 1)
	parsed, err := o.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, centersOf(parsed), test.ShouldResemble, map[string]r3.Vector{"wall": {X: 0, Y: -300, Z: 0}})
	model, err := o.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, model.ModelConfig().Links, test.ShouldHaveLength, 1)

	// Invalid updates are rejected and leave the obstacle untouched.
	for _, cmd := range []map[string]interface{}{
		{CommandKey: CommandSet},
		{CommandKey: CommandSet, GeometriesKey: "wall"},
		{CommandKey: CommandSet, GeometriesKey: []interface{}{}},
		{CommandKey: CommandSet, GeometriesKey: []interface{}{map[string]interface{}{"type": "blob"}}},
		{CommandKey: "bogus"},
		{},
	} {
		_, err = o.DoCommand(ctx, cmd)
		test.That(t, err, test.ShouldNotBeNil)
	}
	parsed, err = o.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, parsed, test.ShouldHaveLength, 1)
}
