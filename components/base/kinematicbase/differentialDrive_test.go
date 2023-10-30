package kinematicbase

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	fakebase "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/fake"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

// Limits when localizer isn't present.
const testNilLocalizerMoveLimit = 10000.

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
	logger := logging.NewTestLogger(t)

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
			ddk, err := buildTestDDK(ctx, testCfg, true, defaultLinearVelocityMMPerSec, defaultAngularVelocityDegsPerSec, logger)
			test.That(t, err == nil, test.ShouldEqual, tc.success)
			if err != nil {
				return
			}
			limits := ddk.executionFrame.DoF()
			test.That(t, limits[0].Min, test.ShouldBeLessThan, 0)
			test.That(t, limits[1].Min, test.ShouldBeLessThan, 0)
			test.That(t, limits[0].Max, test.ShouldBeGreaterThan, 0)
			test.That(t, limits[1].Max, test.ShouldBeGreaterThan, 0)
			geometry, err := ddk.executionFrame.(*referenceframe.SimpleModel).Geometries(make([]referenceframe.Input, len(limits)))
			test.That(t, err, test.ShouldBeNil)
			equivalent := geometry.GeometryByName(testCfg.Name + ":" + testCfg.Frame.Geometry.Label).AlmostEqual(expectedSphere)
			test.That(t, equivalent, test.ShouldBeTrue)
		})
	}

	t.Run("Successful setting of velocities", func(t *testing.T) {
		velocities := []struct {
			linear  float64
			angular float64
		}{
			{10.1, 20.2},
			{0, -1.5},
			{-1.9, 0},
			{-1, 2},
			{3, -4},
		}
		for _, vels := range velocities {
			ddk, err := buildTestDDK(ctx, testConfig(), true, vels.linear, vels.angular, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ddk.options.LinearVelocityMMPerSec, test.ShouldAlmostEqual, vels.linear)
			test.That(t, ddk.options.AngularVelocityDegsPerSec, test.ShouldAlmostEqual, vels.angular)
		}
	})
}

func TestCurrentInputs(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("with Localizer", func(t *testing.T) {
		ddk, err := buildTestDDK(ctx, testConfig(), true,
			defaultLinearVelocityMMPerSec, defaultAngularVelocityDegsPerSec, logger)
		test.That(t, err, test.ShouldBeNil)
		for i := 0; i < 10; i++ {
			_, err := ddk.CurrentInputs(ctx)
			test.That(t, err, test.ShouldBeNil)
		}
	})

	t.Run("without Localizer", func(t *testing.T) {
		ddk, err := buildTestDDK(ctx, testConfig(), false,
			defaultLinearVelocityMMPerSec, defaultAngularVelocityDegsPerSec, logger)
		test.That(t, err, test.ShouldBeNil)
		for i := 0; i < 10; i++ {
			input, err := ddk.CurrentInputs(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, input, test.ShouldResemble, []referenceframe.Input{{Value: 0}, {Value: 0}, {Value: 0}})
		}
	})
}

func TestInputDiff(t *testing.T) {
	ctx := context.Background()

	// make injected slam service
	slam := inject.NewSLAMService("the slammer")
	slam.PositionFunc = func(ctx context.Context) (spatialmath.Pose, string, error) {
		return spatialmath.NewZeroPose(), "", nil
	}

	// build base
	logger := logging.NewTestLogger(t)
	ddk, err := buildTestDDK(ctx, testConfig(), true,
		defaultLinearVelocityMMPerSec, defaultAngularVelocityDegsPerSec, logger)
	test.That(t, err, test.ShouldBeNil)
	ddk.Localizer = motion.NewSLAMLocalizer(slam)

	desiredInput := []referenceframe.Input{{Value: 3}, {Value: 4}, {Value: utils.DegToRad(30)}}
	distErr, headingErr, err := ddk.inputDiff(make([]referenceframe.Input, 3), desiredInput)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, distErr, test.ShouldEqual, r3.Vector{X: desiredInput[0].Value, Y: desiredInput[1].Value, Z: 0}.Norm())
	test.That(t, headingErr, test.ShouldAlmostEqual, 30)
}

func buildTestDDK(
	ctx context.Context,
	cfg resource.Config,
	hasLocalizer bool,
	linVel, angVel float64,
	logger logging.Logger,
) (*differentialDriveKinematics, error) {
	// make fake base
	b, err := fakebase.NewBase(ctx, resource.Dependencies{}, cfg, logger)
	if err != nil {
		return nil, err
	}

	// make a SLAM service and get its limits
	var localizer motion.Localizer
	var limits []referenceframe.Limit
	if hasLocalizer {
		fakeSLAM := fake.NewSLAM(slam.Named("test"), logger)
		limits, err = fakeSLAM.Limits(ctx)
		if err != nil {
			return nil, err
		}
		localizer = motion.NewSLAMLocalizer(fakeSLAM)
	} else {
		limits = []referenceframe.Limit{
			{Min: testNilLocalizerMoveLimit, Max: testNilLocalizerMoveLimit},
			{Min: testNilLocalizerMoveLimit, Max: testNilLocalizerMoveLimit},
		}
	}
	limits = append(limits, referenceframe.Limit{Min: -2 * math.Pi, Max: 2 * math.Pi})

	// construct differential drive kinematic base
	options := NewKinematicBaseOptions()
	options.LinearVelocityMMPerSec = linVel
	options.AngularVelocityDegsPerSec = angVel
	kb, err := wrapWithDifferentialDriveKinematics(ctx, b, logger, localizer, limits, options)
	if err != nil {
		return nil, err
	}
	ddk, ok := kb.(*differentialDriveKinematics)
	if !ok {
		return nil, err
	}
	return ddk, nil
}

func TestNewValidRegionCapsule(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	ddk, err := buildTestDDK(ctx, testConfig(), true, defaultLinearVelocityMMPerSec, defaultAngularVelocityDegsPerSec, logger)
	test.That(t, err, test.ShouldBeNil)

	starting := referenceframe.FloatsToInputs([]float64{400, 0, 0})
	desired := referenceframe.FloatsToInputs([]float64{0, 400, 0})
	c, err := ddk.newValidRegionCapsule(starting, desired)
	test.That(t, err, test.ShouldBeNil)

	col, err := c.CollidesWith(spatialmath.NewPoint(r3.Vector{X: -176, Y: 576, Z: 0}, ""))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeTrue)

	col, err = c.CollidesWith(spatialmath.NewPoint(
		r3.Vector{X: -defaultPlanDeviationThresholdMM, Y: -defaultPlanDeviationThresholdMM, Z: 0},
		"",
	))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, col, test.ShouldBeFalse)
}
