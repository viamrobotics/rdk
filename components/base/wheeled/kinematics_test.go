package wheeled

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam/fake"
	"go.viam.com/rdk/spatialmath"
)

func TestKinematicBase(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	label := "base"
	frame := &referenceframe.LinkConfig{
		Geometry: &spatialmath.GeometryConfig{
			R:                 5,
			X:                 8,
			Y:                 6,
			L:                 10,
			TranslationOffset: r3.Vector{X: 3, Y: 4, Z: 0},
			Label:             label,
		},
	}

	testCases := []struct {
		geoType spatialmath.GeometryType
		success bool
	}{
		{spatialmath.SphereType, true},
		{spatialmath.BoxType, true},
		{spatialmath.CapsuleType, true},
		{spatialmath.UnknownType, true},
		{spatialmath.GeometryType("bad"), false},
	}

	deps, err := testCfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)
	kinematicCfg := testCfg

	expectedSphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "")
	test.That(t, err, test.ShouldBeNil)

	for _, tc := range testCases {
		t.Run(string(tc.geoType), func(t *testing.T) {
			frame.Geometry.Type = tc.geoType
			kinematicCfg.Frame = frame
			basic, err := CreateWheeledBase(ctx, motorDeps, kinematicCfg, logger)
			test.That(t, err, test.ShouldBeNil)
			wb, err := WrapWithKinematics(ctx, basic.(*wheeledBase), fake.NewSLAM("", logger))
			test.That(t, err == nil, test.ShouldEqual, tc.success)
			if err != nil {
				return
			}
			kwb, ok := wb.(*kinematicWheeledBase)
			test.That(t, ok, test.ShouldBeTrue)
			limits := kwb.model.DoF()
			test.That(t, limits[0].Min, test.ShouldBeLessThan, 0)
			test.That(t, limits[1].Min, test.ShouldBeLessThan, 0)
			test.That(t, limits[1].Max, test.ShouldBeGreaterThan, 0)
			test.That(t, limits[1].Max, test.ShouldBeGreaterThan, 0)
			geometry, err := kwb.model.(*referenceframe.SimpleModel).Geometries(make([]referenceframe.Input, len(limits)))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, geometry.GeometryByName(testCfg.Name+":"+label).AlmostEqual(expectedSphere), test.ShouldBeTrue)
		})
	}
}
