package kinematicbase

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	fakebase "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/fake"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func testConfig() resource.Config {
	return resource.Config{
		Name: "test",
		API:  base.API,
		Frame: &referenceframe.LinkConfig{
			Geometry: &spatialmath.GeometryConfig{
				R:                 5,
				X:                 8,
				Y:                 6,
				L:                 10,
				TranslationOffset: r3.Vector{X: 3, Y: 4, Z: 0},
				Label:             "ddk",
			},
		},
	}
}

func TestWrapWithDifferentialDriveKinematics(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

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

	expectedSphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "")
	test.That(t, err, test.ShouldBeNil)

	for _, tc := range testCases {
		t.Run(string(tc.geoType), func(t *testing.T) {
			testCfg := testConfig()
			testCfg.Frame.Geometry.Type = tc.geoType
			ddk, err := buildTestDDK(ctx, testCfg, logger)
			test.That(t, err == nil, test.ShouldEqual, tc.success)
			if err != nil {
				return
			}
			limits := ddk.model.DoF()
			test.That(t, limits[0].Min, test.ShouldBeLessThan, 0)
			test.That(t, limits[1].Min, test.ShouldBeLessThan, 0)
			test.That(t, limits[0].Max, test.ShouldBeGreaterThan, 0)
			test.That(t, limits[1].Max, test.ShouldBeGreaterThan, 0)
			geometry, err := ddk.model.(*referenceframe.SimpleModel).Geometries(make([]referenceframe.Input, len(limits)))
			test.That(t, err, test.ShouldBeNil)
			equivalent := geometry.GeometryByName(testCfg.Name + ":" + testCfg.Frame.Geometry.Label).AlmostEqual(expectedSphere)
			test.That(t, equivalent, test.ShouldBeTrue)
		})
	}
}

func TestCurrentInputs(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	ddk, err := buildTestDDK(ctx, testConfig(), logger)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 10; i++ {
		_, err := ddk.CurrentInputs(ctx)
		test.That(t, err, test.ShouldBeNil)
	}
}

func TestErrorState(t *testing.T) {
	ctx := context.Background()

	// make injected slam service
	slam := inject.NewSLAMService("the slammer")
	slam.GetPositionFunc = func(ctx context.Context) (spatialmath.Pose, string, error) {
		return spatialmath.NewZeroPose(), "", nil
	}
	localizer, err := motion.NewLocalizer(ctx, slam)
	test.That(t, err, test.ShouldBeNil)

	// build base
	logger := golog.NewTestLogger(t)
	ddk, err := buildTestDDK(ctx, testConfig(), logger)
	test.That(t, err, test.ShouldBeNil)
	ddk.localizer = localizer

	desiredInput := []referenceframe.Input{{3}, {4}, {utils.DegToRad(30)}}
	distErr, headingErr, err := ddk.errorState(make([]referenceframe.Input, 3), desiredInput)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, distErr, test.ShouldEqual, r3.Vector{desiredInput[0].Value, desiredInput[1].Value, 0}.Norm())
	test.That(t, headingErr, test.ShouldAlmostEqual, 30)
}

func buildTestDDK(ctx context.Context, cfg resource.Config, logger golog.Logger) (*differentialDriveKinematics, error) {
	// make fake base
	b, err := fakebase.NewBase(ctx, resource.Dependencies{}, cfg, logger)
	if err != nil {
		return nil, err
	}
	geometries, err := CollisionGeometry(cfg.Frame)
	if err != nil {
		return nil, err
	}
	b.(*fakebase.Base).Geometry = geometries

	// make a SLAM service and get its limits
	fakeSLAM := fake.NewSLAM(slam.Named("test"), logger)
	limits, err := fakeSLAM.GetLimits(ctx)
	if err != nil {
		return nil, err
	}

	// construct localizer
	localizer, err := motion.NewLocalizer(ctx, fakeSLAM)
	if err != nil {
		return nil, err
	}

	kb, err := WrapWithDifferentialDriveKinematics(ctx, b, localizer, limits)
	if err != nil {
		return nil, err
	}

	ddk, ok := kb.(*differentialDriveKinematics)
	if !ok {
		return nil, err
	}
	return ddk, nil
}
