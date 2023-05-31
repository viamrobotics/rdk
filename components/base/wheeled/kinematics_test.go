package wheeled

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/fake"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestWrapWithKinematics(t *testing.T) {
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

	testCfg := newTestCfg()
	deps, err := testCfg.Validate("path", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)
	motorDeps := fakeMotorDependencies(t, deps)
	kinematicCfg := testCfg

	expectedSphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "")
	test.That(t, err, test.ShouldBeNil)

	for _, tc := range testCases {
		t.Run(string(tc.geoType), func(t *testing.T) {
			frame.Geometry.Type = tc.geoType
			kinematicCfg.Frame = frame
			basic, err := createWheeledBase(ctx, motorDeps, kinematicCfg, logger)
			test.That(t, err, test.ShouldBeNil)
			wb, err := basic.(*wheeledBase).WrapWithKinematics(ctx, fake.NewSLAM(slam.Named("foo"), logger))
			test.That(t, err == nil, test.ShouldEqual, tc.success)
			if err != nil {
				return
			}
			kwb, ok := wb.(*kinematicWheeledBase)
			test.That(t, ok, test.ShouldBeTrue)
			limits := kwb.model.DoF()
			test.That(t, limits[0].Min, test.ShouldBeLessThan, 0)
			test.That(t, limits[1].Min, test.ShouldBeLessThan, 0)
			test.That(t, limits[0].Max, test.ShouldBeGreaterThan, 0)
			test.That(t, limits[1].Max, test.ShouldBeGreaterThan, 0)
			geometry, err := kwb.model.(*referenceframe.SimpleModel).Geometries(make([]referenceframe.Input, len(limits)))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, geometry.GeometryByName(testCfg.Name+":"+label).AlmostEqual(expectedSphere), test.ShouldBeTrue)
		})
	}
}

func newWheeledBase(ctx context.Context, t *testing.T, logger golog.Logger) *kinematicWheeledBase {
	t.Helper()
	wb := &wheeledBase{
		widthMm:              400,
		wheelCircumferenceMm: 25,
		logger:               logger,
		name:                 "count basie",
		frame: &referenceframe.LinkConfig{
			Parent: referenceframe.World,
			Geometry: &spatialmath.GeometryConfig{
				R: 400,
			},
		},
	}
	kb, err := wb.WrapWithKinematics(ctx, fake.NewSLAM(resource.NewName(slam.API, "foo"), golog.NewTestLogger(t)))
	test.That(t, err, test.ShouldBeNil)
	kwb, ok := kb.(*kinematicWheeledBase)
	test.That(t, ok, test.ShouldBeTrue)
	return kwb
}

func TestCurrentInputs(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	kwb := newWheeledBase(ctx, t, logger)

	for i := 0; i < 10; i++ {
		_, err := kwb.CurrentInputs(ctx)
		test.That(t, err, test.ShouldBeNil)
		_, err = slam.GetPointCloudMapFull(ctx, kwb.slam)
		test.That(t, err, test.ShouldBeNil)
	}
}

func TestErrorState(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	wb := newWheeledBase(ctx, t, logger).wheeledBase
	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 1, "")
	test.That(t, err, test.ShouldBeNil)
	model, err := referenceframe.New2DMobileModelFrame(wb.name, []referenceframe.Limit{{-10, 10}, {-10, 10}, {-math.Pi, math.Pi}}, sphere)
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("")
	test.That(t, fs.AddFrame(model, fs.World()), test.ShouldBeNil)

	slam := inject.NewSLAMService("the slammer")
	slam.GetPositionFunc = func(ctx context.Context) (spatialmath.Pose, string, error) {
		return spatialmath.NewZeroPose(), "", nil
	}
	kwb := &kinematicWheeledBase{
		wheeledBase: wb,
		slam:        slam,
		model:       model,
		fs:          fs,
	}
	desiredInput := []referenceframe.Input{{3}, {4}, {utils.DegToRad(30)}}
	distErr, headingErr, err := kwb.errorState(make([]referenceframe.Input, 3), desiredInput)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, distErr, test.ShouldEqual, 1000*r3.Vector{desiredInput[0].Value, desiredInput[1].Value, 0}.Norm())
	test.That(t, headingErr, test.ShouldAlmostEqual, 30)
}
