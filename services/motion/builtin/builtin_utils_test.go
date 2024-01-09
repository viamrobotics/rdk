package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/base"
	baseFake "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func setupMotionServiceFromConfig(t *testing.T, configFilename string) (motion.Service, func()) {
	t.Helper()
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(ctx, configFilename, logger)
	test.That(t, err, test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	svc, err := motion.FromRobot(myRobot, "builtin")
	test.That(t, err, test.ShouldBeNil)
	return svc, func() {
		myRobot.Close(context.Background())
	}
}

func getPointCloudMap(path string) (func() ([]byte, error), error) {
	const chunkSizeBytes = 1 * 1024 * 1024
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	chunk := make([]byte, chunkSizeBytes)
	f := func() ([]byte, error) {
		bytesRead, err := file.Read(chunk)
		if err != nil {
			defer utils.UncheckedErrorFunc(file.Close)
			return nil, err
		}
		return chunk[:bytesRead], err
	}
	return f, nil
}

func createInjectedMovementSensor(name string, gpsPoint *geo.Point) *inject.MovementSensor {
	injectedMovementSensor := inject.NewMovementSensor(name)
	injectedMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return gpsPoint, 0, nil
	}
	injectedMovementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, nil
	}
	injectedMovementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{CompassHeadingSupported: true}, nil
	}

	return injectedMovementSensor
}

func createInjectedSlam(name, pcdPath string, origin spatialmath.Pose) *inject.SLAMService {
	injectSlam := inject.NewSLAMService(name)
	injectSlam.PointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		return getPointCloudMap(filepath.Clean(artifact.MustPath(pcdPath)))
	}
	injectSlam.PositionFunc = func(ctx context.Context) (spatialmath.Pose, string, error) {
		if origin != nil {
			return origin, "", nil
		}
		return spatialmath.NewZeroPose(), "", nil
	}
	return injectSlam
}

func createBaseLink(t *testing.T) *referenceframe.LinkInFrame {
	basePose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
	baseSphere, err := spatialmath.NewSphere(basePose, 10, "base-sphere")
	test.That(t, err, test.ShouldBeNil)
	baseLink := referenceframe.NewLinkInFrame(
		referenceframe.World,
		spatialmath.NewZeroPose(),
		"test-base",
		baseSphere,
	)
	return baseLink
}

func createFrameSystemService(
	ctx context.Context,
	deps resource.Dependencies,
	fsParts []*referenceframe.FrameSystemPart,
	logger logging.Logger,
) (framesystem.Service, error) {
	fsSvc, err := framesystem.New(ctx, deps, logger)
	if err != nil {
		return nil, err
	}
	conf := resource.Config{
		ConvertedAttributes: &framesystem.Config{Parts: fsParts},
	}
	if err := fsSvc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	deps[fsSvc.Name()] = fsSvc

	return fsSvc, nil
}

func createMoveOnGlobeEnvironment(ctx context.Context, t *testing.T, origin *geo.Point, noise spatialmath.Pose, sleepTime int) (
	*inject.MovementSensor, framesystem.Service, kinematicbase.KinematicBase, motion.Service,
) {
	logger := logging.NewTestLogger(t)

	// create fake base
	baseCfg := resource.Config{
		Name:  "test-base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 40}},
	}
	fakeBase, err := baseFake.NewBase(ctx, nil, baseCfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// create base link
	baseLink := createBaseLink(t)
	// create MovementSensor link
	movementSensorLink := referenceframe.NewLinkInFrame(
		baseLink.Name(),
		spatialmath.NewPoseFromPoint(r3.Vector{X: -10, Y: 0, Z: 0}),
		"test-gps",
		nil,
	)

	// create a fake kinematic base
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()
	kinematicsOptions.PlanDeviationThresholdMM = 1 // can afford to do this for tests
	kb, err := kinematicbase.WrapWithFakePTGKinematics(
		ctx,
		fakeBase.(*baseFake.Base),
		logger,
		referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewZeroPose()),
		kinematicsOptions,
		noise,
		sleepTime,
	)
	test.That(t, err, test.ShouldBeNil)

	// create injected MovementSensor
	dynamicMovementSensor := inject.NewMovementSensor("test-gps")
	dynamicMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		poseInFrame, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		heading := poseInFrame.Pose().Orientation().OrientationVectorDegrees().Theta
		distance := poseInFrame.Pose().Point().Norm()
		pt := origin.PointAtDistanceAndBearing(distance*1e-6, heading)
		return pt, 0, nil
	}
	dynamicMovementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, nil
	}
	dynamicMovementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{CompassHeadingSupported: true}, nil
	}

	// create injected vision service
	injectedVisionSvc := inject.NewVisionService("injectedVisionSvc")

	cameraGeom, err := spatialmath.NewBox(
		spatialmath.NewZeroPose(),
		r3.Vector{X: 1, Y: 1, Z: 1}, "camera",
	)
	test.That(t, err, test.ShouldBeNil)

	injectedCamera := inject.NewCamera("injectedCamera")
	cameraLink := referenceframe.NewLinkInFrame(
		baseLink.Name(),
		spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 0, Z: 0}),
		"injectedCamera",
		cameraGeom,
	)

	// create the frame system
	fsParts := []*referenceframe.FrameSystemPart{
		{FrameConfig: movementSensorLink},
		{FrameConfig: baseLink},
		{FrameConfig: cameraLink},
	}
	deps := resource.Dependencies{
		fakeBase.Name():              kb,
		dynamicMovementSensor.Name(): dynamicMovementSensor,
		injectedVisionSvc.Name():     injectedVisionSvc,
		injectedCamera.Name():        injectedCamera,
	}

	fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
	test.That(t, err, test.ShouldBeNil)

	conf := resource.Config{ConvertedAttributes: &Config{}}
	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)

	return dynamicMovementSensor, fsSvc, kb, ms
}

func createMoveOnMapEnvironment(
	ctx context.Context,
	t *testing.T,
	pcdPath string,
	geomSize float64,
	origin spatialmath.Pose,
) (kinematicbase.KinematicBase, motion.Service) {
	if origin == nil {
		origin = spatialmath.NewZeroPose()
	}
	injectSlam := createInjectedSlam("test_slam", pcdPath, origin)

	baseLink := createBaseLink(t)

	cfg := resource.Config{
		Name:  "test-base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: geomSize}},
	}
	logger := logging.NewTestLogger(t)
	fakeBase, err := baseFake.NewBase(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()
	kinematicsOptions.PlanDeviationThresholdMM = 1 // can afford to do this for tests
	kb, err := kinematicbase.WrapWithFakePTGKinematics(
		ctx,
		fakeBase.(*baseFake.Base),
		logger,
		referenceframe.NewPoseInFrame(referenceframe.World, origin),
		kinematicsOptions,
		spatialmath.NewZeroPose(),
		50,
	)
	test.That(t, err, test.ShouldBeNil)

	deps := resource.Dependencies{injectSlam.Name(): injectSlam, fakeBase.Name(): kb}
	conf := resource.Config{ConvertedAttributes: &Config{}}

	// create the frame system
	fsParts := []*referenceframe.FrameSystemPart{
		{FrameConfig: baseLink},
	}

	_, err = createFrameSystemService(ctx, deps, fsParts, logger)
	test.That(t, err, test.ShouldBeNil)

	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	return kb, ms
}
