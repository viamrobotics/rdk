package builtin

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.uber.org/multierr"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/base/wheeled"
	"go.viam.com/rdk/components/encoder"
	fakeencoder "go.viam.com/rdk/components/encoder/fake"
	"go.viam.com/rdk/components/motor"
	fakemotor "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/wheeledodometry"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

var (
	leftMotorName  = "leftmotor"
	rightMotorName = "rightmotor"
	baseName       = "test-base"
	moveSensorName = "test-movement-sensor"

	moveSensorResource        = resource.NewName(movementsensor.API, moveSensorName)
	baseResource              = resource.NewName(base.API, baseName)
	movementSensorInBasePoint = r3.Vector{X: -10, Y: 0, Z: 0}
	updateRate                = 33
)

func getPointCloudMap(path string) (func() ([]byte, error), error) {
	const chunkSizeBytes = 1 * 1024 * 1024
	file, err := os.Open(path) //nolint:gosec
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

func createInjectedSlam(name string) *inject.SLAMService {
	pcdPath := "pointcloud/octagonspace.pcd"
	injectSlam := inject.NewSLAMService(name)
	injectSlam.PointCloudMapFunc = func(ctx context.Context, returnEditedMap bool) (func() ([]byte, error), error) {
		return getPointCloudMap(filepath.Clean(artifact.MustPath(pcdPath)))
	}
	injectSlam.PositionFunc = func(ctx context.Context) (spatialmath.Pose, error) {
		return spatialmath.NewZeroPose(), nil
	}
	injectSlam.PropertiesFunc = func(ctx context.Context) (slam.Properties, error) {
		return slam.Properties{
			CloudSlam:             false,
			MappingMode:           slam.MappingModeLocalizationOnly,
			InternalStateFileType: ".pbstream",
			SensorInfo: []slam.SensorInfo{
				{Name: "my-camera", Type: slam.SensorTypeCamera},
				{Name: "my-movement-sensor", Type: slam.SensorTypeMovementSensor},
			},
		}, nil
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
		baseName,
		baseSphere,
	)
	return baseLink
}

func createDependencies(
	t *testing.T,
	ctx context.Context,
	logger logging.Logger,
	geomSize float64,
	origin *geo.Point,
	noise spatialmath.Pose,
) resource.Dependencies {
	encoders := createEncoders(t, ctx, logger)
	motors := createMotors(t, ctx, logger, encoders)
	fakeBase := createWheeledBase(t, ctx, logger, motors, geomSize)
	sens := createMovementSensor(t, ctx, logger, motors, fakeBase, origin, noise)

	deps := resource.Dependencies{}
	deps[motors[leftMotorName].Name()] = motors[leftMotorName]
	deps[motors[rightMotorName].Name()] = motors[rightMotorName]
	deps[fakeBase.Name()] = fakeBase
	deps[sens.Name()] = sens

	return deps
}

func createWheeledBase(
	t *testing.T,
	ctx context.Context,
	logger logging.Logger,
	motors map[string]motor.Motor,
	geomSize float64,
) base.Base {
	t.Helper()

	reg, ok := resource.LookupRegistration(base.API, wheeled.Model)
	test.That(t, ok, test.ShouldBeTrue)

	wheeledConf := &wheeled.Config{
		WidthMM:              200,
		WheelCircumferenceMM: 2000,
		Left:                 []string{leftMotorName},
		Right:                []string{rightMotorName},
	}

	baseConf := resource.Config{
		Name:                baseName,
		API:                 base.API,
		Frame:               &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: geomSize}},
		ConvertedAttributes: wheeledConf,
	}
	deps := resource.Dependencies{}
	deps[motors[leftMotorName].Name()] = motors[leftMotorName]
	deps[motors[rightMotorName].Name()] = motors[rightMotorName]

	wBase, err := reg.Constructor(ctx, deps, baseConf, logger)
	test.That(t, err, test.ShouldBeNil)
	return wBase.(base.Base)
}

func createEncoders(t *testing.T, ctx context.Context, logger logging.Logger) map[string]encoder.Encoder {
	t.Helper()
	leftconf := resource.Config{
		Name:                leftMotorName + "_encoder",
		API:                 encoder.API,
		ConvertedAttributes: &fakeencoder.Config{UpdateRate: int64(updateRate)},
	}
	rightconf := resource.Config{
		Name:                rightMotorName + "_encoder",
		API:                 encoder.API,
		ConvertedAttributes: &fakeencoder.Config{UpdateRate: int64(updateRate)},
	}
	left, err := fakeencoder.NewEncoder(ctx, leftconf, logger)
	test.That(t, err, test.ShouldBeNil)
	right, err := fakeencoder.NewEncoder(ctx, rightconf, logger)
	test.That(t, err, test.ShouldBeNil)
	return map[string]encoder.Encoder{leftMotorName: left, rightMotorName: right}
}

func createMotors(
	t *testing.T,
	ctx context.Context,
	logger logging.Logger,
	encoders map[string]encoder.Encoder,
) map[string]motor.Motor {
	t.Helper()
	motorMap := map[string]motor.Motor{}
	for name, encoder := range encoders {
		conf := resource.Config{
			Name:                name,
			API:                 motor.API,
			ConvertedAttributes: &fakemotor.Config{Encoder: name + "_encoder", TicksPerRotation: 4000},
		}

		thisMotor, err := fakemotor.NewMotor(
			ctx,
			resource.Dependencies{encoder.Name(): encoder},
			conf,
			logger,
		)
		test.That(t, err, test.ShouldBeNil)
		motorMap[name] = thisMotor
	}
	return motorMap
}

type noisyMovementSensor struct {
	movementsensor.MovementSensor
	mu         sync.Mutex
	noise      spatialmath.Pose
	queryCount int // Apply noise every other iteration when this is > 1
}

func (m *noisyMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCount++
	// We cannot have a single offset, as the first replan will correct for that noise and a second replan will not occur.
	// Thus we must oscillate whether or not we return a pose with the noise applied.
	if m.queryCount <= 10 || m.queryCount%2 == 0 {
		return m.MovementSensor.Position(ctx, extra)
	}
	pos, alt, err := m.MovementSensor.Position(ctx, extra)
	if err != nil {
		return nil, 0, err
	}
	newPos := spatialmath.PoseToGeoPose(spatialmath.NewGeoPose(pos, 0), m.noise)
	return newPos.Location(), alt, nil
}

func (m *noisyMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.queryCount <= 10 || m.queryCount%2 == 0 {
		return m.MovementSensor.CompassHeading(ctx, extra)
	}
	heading, err := m.MovementSensor.CompassHeading(ctx, extra)
	if err != nil {
		return 0, err
	}
	newPos := spatialmath.PoseToGeoPose(spatialmath.NewGeoPose(geo.NewPoint(0, 0), heading), m.noise)
	return math.Mod(newPos.Heading(), 360), nil
}

func createMovementSensor(
	t *testing.T,
	ctx context.Context,
	logger logging.Logger,
	motors map[string]motor.Motor,
	fakeBase base.Base,
	origin *geo.Point,
	noise spatialmath.Pose,
) movementsensor.MovementSensor {
	t.Helper()
	reg, ok := resource.LookupRegistration(movementsensor.API, wheeledodometry.Model)
	test.That(t, ok, test.ShouldBeTrue)

	odoConf := &wheeledodometry.Config{
		LeftMotors:        []string{leftMotorName},
		RightMotors:       []string{rightMotorName},
		Base:              fakeBase.Name().ShortName(),
		TimeIntervalMSecs: float64(updateRate),
	}

	moveConf := resource.Config{
		Name:                moveSensorName,
		API:                 movementsensor.API,
		ConvertedAttributes: odoConf,
	}
	deps := resource.Dependencies{}
	deps[motors[leftMotorName].Name()] = motors[leftMotorName]
	deps[motors[rightMotorName].Name()] = motors[rightMotorName]
	deps[fakeBase.Name()] = fakeBase

	sens, err := reg.Constructor(ctx, deps, moveConf, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = sens.DoCommand(ctx, map[string]interface{}{"use_compass": true})
	test.That(t, err, test.ShouldBeNil)

	_, err = sens.DoCommand(ctx, map[string]interface{}{"reset": true, "setLong": origin.Lng(), "setLat": origin.Lat()})
	test.That(t, err, test.ShouldBeNil)
	if noise != nil {
		sens = &noisyMovementSensor{
			MovementSensor: sens.(movementsensor.MovementSensor),
			noise:          noise,
			queryCount:     0,
		}
	}

	return sens.(movementsensor.MovementSensor)
}

func createMockSlamService(name, pcdPath string, origin spatialmath.Pose, move movementsensor.MovementSensor) (*inject.SLAMService, error) {
	injectSlam := inject.NewSLAMService(name)
	injectSlam.PointCloudMapFunc = func(ctx context.Context, returnEditedMap bool) (func() ([]byte, error), error) {
		return getPointCloudMap(filepath.Clean(artifact.MustPath(pcdPath)))
	}

	gpsOrigin, _, err := move.Position(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	if origin == nil {
		origin = spatialmath.NewZeroPose()
	}

	injectSlam.PositionFunc = func(ctx context.Context) (spatialmath.Pose, error) {
		currentGPS, _, err := move.Position(ctx, nil)
		if err != nil {
			return nil, err
		}
		headingLeft, err := move.CompassHeading(ctx, nil)
		if err != nil {
			return nil, err
		}
		// Adjust for proper x-y control
		headingLeft -= 90
		// CompassHeading is a left-handed value. Convert to be right-handed. Use math.Mod to ensure that 0 reports 0 rather than 360.
		heading := math.Mod(math.Abs(headingLeft-360), 360)
		orientation := &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: heading}

		geoPose := spatialmath.NewPose(spatialmath.GeoPointToPoint(currentGPS, gpsOrigin), orientation)

		return spatialmath.Compose(origin, geoPose), nil
	}
	injectSlam.PropertiesFunc = func(ctx context.Context) (slam.Properties, error) {
		return slam.Properties{
			CloudSlam:             false,
			MappingMode:           slam.MappingModeLocalizationOnly,
			InternalStateFileType: ".pbstream",
			SensorInfo: []slam.SensorInfo{
				{Name: "my-camera", Type: slam.SensorTypeCamera},
				{Name: "my-movement-sensor", Type: slam.SensorTypeMovementSensor},
			},
		}, nil
	}
	return injectSlam, nil
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

// CreateMoveOnGlobeTestEnvironment creates a testable environment that will simulate a real moving base for MoveOnGlobe calls.
func CreateMoveOnGlobeTestEnvironment(ctx context.Context, t *testing.T, origin *geo.Point, geometrySize float64, noise spatialmath.Pose) (
	motion.Localizer, motion.Service, func(context.Context) error,
) {
	ctx, cFunc := context.WithCancel(ctx)
	logger := logging.NewTestLogger(t)

	// create fake wheeled base
	deps := createDependencies(t, ctx, logger, geometrySize, origin, noise)
	movementSensor, err := movementsensor.FromDependencies(deps, moveSensorName)
	test.That(t, err, test.ShouldBeNil)
	// create base link
	baseLink := createBaseLink(t)
	// create MovementSensor link
	movementSensorLink := referenceframe.NewLinkInFrame(
		baseLink.Name(),
		spatialmath.NewPoseFromPoint(movementSensorInBasePoint),
		"test-gps",
		nil,
	)

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
		spatialmath.NewPose(r3.Vector{X: 1}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90}),
		"injectedCamera",
		cameraGeom,
	)

	// create the frame system
	fsParts := []*referenceframe.FrameSystemPart{
		{FrameConfig: movementSensorLink},
		{FrameConfig: baseLink},
		{FrameConfig: cameraLink},
	}

	deps[injectedVisionSvc.Name()] = injectedVisionSvc
	deps[injectedCamera.Name()] = injectedCamera

	fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
	test.That(t, err, test.ShouldBeNil)

	conf := resource.Config{ConvertedAttributes: &Config{}}
	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	ms.(*builtIn).fsService = fsSvc

	startPosition, _, err := movementSensor.Position(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)

	localizer := motion.NewMovementSensorLocalizer(movementSensor, startPosition, nil)

	closeFunc := func(ctx context.Context) error {
		err := multierr.Combine(movementSensor.Close(ctx), ms.Close(ctx))
		cFunc()
		// wait for closing to finish
		time.Sleep(50 * time.Millisecond)
		return err
	}

	return localizer, ms, closeFunc
}

// CreateMoveOnMapTestEnvironment creates a testable environment that will simulate a real moving base for MoveOnMap calls.
func CreateMoveOnMapTestEnvironment(
	ctx context.Context,
	t *testing.T,
	pcdPath string,
	geomSize float64,
	origin spatialmath.Pose,
) (motion.Localizer, motion.Service, func(context.Context) error) {
	ctx, cFunc := context.WithCancel(ctx)
	if origin == nil {
		origin = motion.SLAMOrientationAdjustment
	}
	logger := logging.NewTestLogger(t)

	deps := createDependencies(t, ctx, logger, geomSize, &geo.Point{}, nil)

	movementSensor, err := movementsensor.FromDependencies(deps, moveSensorName)
	test.That(t, err, test.ShouldBeNil)
	injectSlam, err := createMockSlamService("test_slam", pcdPath, origin, movementSensor)
	test.That(t, err, test.ShouldBeNil)
	injectVis := inject.NewVisionService("test-vision")
	injectCam := inject.NewCamera("test-camera")
	baseLink := createBaseLink(t)

	deps[injectVis.Name()] = injectVis
	deps[injectCam.Name()] = injectCam
	deps[injectSlam.Name()] = injectSlam
	conf := resource.Config{ConvertedAttributes: &Config{}}

	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)

	// create the frame system
	cameraGeom, err := spatialmath.NewBox(
		spatialmath.NewZeroPose(),
		r3.Vector{X: 1, Y: 1, Z: 1}, "camera",
	)
	test.That(t, err, test.ShouldBeNil)
	cameraLink := referenceframe.NewLinkInFrame(
		baseLink.Name(),
		// we recreate an intel real sense orientation placed along the +Y axis of the base's coordinate frame.
		// i.e. the camera is pointed along the axis in which the base moves forward
		spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{OY: 1, Theta: -90}),
		"test-camera",
		cameraGeom,
	)

	fsParts := []*referenceframe.FrameSystemPart{
		{FrameConfig: baseLink},
		{FrameConfig: cameraLink},
	}

	fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
	test.That(t, err, test.ShouldBeNil)
	ms.(*builtIn).fsService = fsSvc

	// Wheeled odometry returns a movement sensor that reports its position in GPS coordinates. We want to mock up a SLAM service which
	// converts that to a pose.
	localizer := motion.NewSLAMLocalizer(injectSlam)
	closeFunc := func(ctx context.Context) error {
		err := multierr.Combine(movementSensor.Close(ctx), ms.Close(ctx))
		cFunc()
		// wait for closing to finish
		time.Sleep(50 * time.Millisecond)
		return err
	}
	return localizer, ms, closeFunc
}

func createTestKinematicBase(ctx context.Context, t *testing.T) (
	kinematicbase.KinematicBase, func(context.Context) error,
) {
	logger := logging.NewTestLogger(t)

	// create fake wheeled base
	deps := createDependencies(t, ctx, logger, 80, &geo.Point{}, nil)
	movementSensor, err := movementsensor.FromDependencies(deps, moveSensorName)
	test.That(t, err, test.ShouldBeNil)
	b, err := base.FromDependencies(deps, baseName)
	test.That(t, err, test.ShouldBeNil)

	startPosition, _, err := movementSensor.Position(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)

	localizer := motion.NewMovementSensorLocalizer(movementSensor, startPosition, nil)
	closeFunc := func(ctx context.Context) error {
		return movementSensor.Close(ctx)
	}
	kbo := kinematicbase.NewKinematicBaseOptions()
	kbo.NoSkidSteer = true
	kbo.UpdateStepSeconds = 0.1

	kb, err := kinematicbase.WrapWithKinematics(ctx, b, logger, localizer, nil, kbo)
	test.That(t, err, test.ShouldBeNil)

	return kb, closeFunc
}
