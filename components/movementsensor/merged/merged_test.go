package merged

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testName = "testSensor"
)

var (
	testlinvel   = r3.Vector{X: 1, Y: 2, Z: 3}
	testori      = &spatialmath.OrientationVector{OX: 0, OY: 0, OZ: -1, Theta: 75}
	testgeopoint = geo.NewPoint(43.4, -72.9)
	testalt      = 45.0
	testcompass  = 75.0
	testangvel   = spatialmath.AngularVelocity{X: 4, Y: 5, Z: 6}
	testlinacc   = r3.Vector{X: 7, Y: 8, Z: 9}

	errAccuracy = errors.New("no accuracy for you merged sensor")
	errProps    = errors.New("no properties for you merged sensor")

	emptySensors   = []string{}
	posSensors     = []string{"goodPos", "unusedPos"}
	oriSensors     = []string{"goodOri", "unusedOri"}
	compassSensors = []string{"badCompass", "goodCompass", "unusedCompass"}
	linvelSensors  = []string{"goodLinVel", "unusedLinVel"}
	angvelSensors  = []string{"goodAngVel", "unusedAngVel"}
	linaccSensors  = []string{"goodLinAcc", "unusedLinAcc"}

	emptyProps   = movementsensor.Properties{}
	oriProps     = movementsensor.Properties{OrientationSupported: true}
	posProps     = movementsensor.Properties{PositionSupported: true}
	compassProps = movementsensor.Properties{CompassHeadingSupported: true}
	linvelProps  = movementsensor.Properties{LinearVelocitySupported: true}
	angvelProps  = movementsensor.Properties{AngularVelocitySupported: true}
	linaccProps  = movementsensor.Properties{LinearAccelerationSupported: true}
)

func setUpCfg(ori, pos, compass, linvel, angvel, linacc []string) resource.Config {
	return resource.Config{
		Name:  testName,
		Model: model,
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			Orientation:        ori,
			Position:           pos,
			CompassHeading:     compass,
			LinearVelocity:     linvel,
			AngularVelocity:    angvel,
			LinearAcceleration: linacc,
		},
	}
}

func setupMovementSensor(
	name string, prop movementsensor.Properties,
	errAcc, oriPropsErr bool,
) movementsensor.MovementSensor {
	ms := inject.NewMovementSensor(name)
	ms.PropertiesFunc = func(ctx context.Context, extra map[string]interface{},
	) (*movementsensor.Properties, error) {
		if oriPropsErr && strings.Contains(name, "goodOri") {
			return &prop, errProps
		}
		return &prop, nil
	}
	ms.AccuracyFunc = func(ctx context.Context, exta map[string]interface{}) (map[string]float32,
		float32, float32, movementsensor.NmeaGGAFixType, float32, error,
	) {
		if errAcc {
			return nil, 0, 0, -1, 0, errAccuracy
		}
		return map[string]float32{"accuracy": 32}, 0, 0, -1, 0, nil
	}

	switch {
	case strings.Contains(name, "Ori"):
		ms.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
			return testori, nil
		}
	case strings.Contains(name, "Pos"):
		ms.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
			return testgeopoint, testalt, nil
		}
	case strings.Contains(name, "Compass"):
		ms.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return testcompass, nil
		}
	case strings.Contains(name, "AngVel"):
		ms.AngularVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
			return testangvel, nil
		}
	case strings.Contains(name, "LinVel"):
		ms.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
			return testlinvel, nil
		}
	case strings.Contains(name, "LinAcc"):
		ms.LinearAccelerationFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
			return testlinacc, nil
		}
	}

	return ms
}

func setupDependencies(t *testing.T,
	depswithProps map[string]movementsensor.Properties,
	errAcc, errProps bool,
) resource.Dependencies {
	t.Helper()
	result := make(resource.Dependencies)
	for name, prop := range depswithProps {
		ms := setupMovementSensor(name, prop, errAcc, errProps)
		result[movementsensor.Named(name)] = ms
	}
	return result
}

func TestValidate(t *testing.T) {
	// doesn't pass validate
	conf := setUpCfg(
		emptySensors, emptySensors /*pos*/, emptySensors, /*compass*/
		emptySensors /*linvel*/, emptySensors /*angvel*/, emptySensors /*linacc*/)
	implicits, err := conf.Validate("somepath", movementsensor.API.Type.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, implicits, test.ShouldBeNil)

	// doesn't pass configuration
	conf = setUpCfg(
		oriSensors, emptySensors /*pos*/, emptySensors, /*compass*/
		linvelSensors, emptySensors /*angvel*/, emptySensors /*linacc*/)
	implicits, err = conf.Validate("somepath", movementsensor.API.Type.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, implicits, test.ShouldResemble, append(oriSensors, linvelSensors...))

	conf = setUpCfg(
		/*ori*/ emptySensors, emptySensors /*pos*/, emptySensors, /*comapss*/
		linvelSensors /*linval*/, angvelSensors /*angvel*/, emptySensors /*linacc*/)
	implicits, err = conf.Validate("somepath", movementsensor.API.Type.Name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, implicits, test.ShouldResemble, append(linvelSensors, angvelSensors...))
}

func TestCreation(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// doesn't pass configuration
	conf := setUpCfg(
		oriSensors, emptySensors /*pos*/, emptySensors, /*compass*/
		linvelSensors, emptySensors /*angvel*/, emptySensors /*linacc*/)

	depmap := map[string]movementsensor.Properties{
		linvelSensors[0]: emptyProps,                       // empty
		linvelSensors[1]: {AngularVelocitySupported: true}, // wrong properties
		oriSensors[0]:    {OrientationSupported: true},     // has correct properties but errors
		oriSensors[1]:    {LinearVelocitySupported: true},  // second sensor in a list, but wrong property
	}

	deps := setupDependencies(t, depmap, false, true /* properties error */)
	ms, err := newMergedModel(ctx, deps, conf, logger)
	test.That(t, err.Error(), test.ShouldContainSubstring, "orientation not supported")
	test.That(t, ms, test.ShouldBeNil)

	// first time passing configuration with two merged sensors
	conf = setUpCfg(
		/*ori*/ emptySensors, emptySensors /*pos*/, emptySensors, /*comapss*/
		linvelSensors /*linval*/, angvelSensors /*angvel*/, emptySensors /*linacc*/)

	depmap = map[string]movementsensor.Properties{
		linvelSensors[0]: linvelProps,
		linvelSensors[1]: linvelProps,
		angvelSensors[0]: angvelProps,
		angvelSensors[1]: angvelProps,
	}

	deps = setupDependencies(t, depmap, false, false)

	ms, err = newMergedModel(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)

	compass, err := ms.CompassHeading(ctx, nil)
	test.That(t, err, test.ShouldBeError, movementsensor.ErrMethodUnimplementedCompassHeading)
	test.That(t, math.IsNaN(compass), test.ShouldBeTrue)

	pos, alt, err := ms.Position(ctx, nil)
	test.That(t, err, test.ShouldBeError, movementsensor.ErrMethodUnimplementedPosition)
	test.That(t, math.IsNaN(pos.Lat()), test.ShouldBeTrue)
	test.That(t, math.IsNaN(pos.Lng()), test.ShouldBeTrue)
	test.That(t, math.IsNaN(alt), test.ShouldBeTrue)

	linacc, err := ms.LinearAcceleration(ctx, nil)
	test.That(t, err, test.ShouldBeError, movementsensor.ErrMethodUnimplementedLinearAcceleration)
	test.That(t, math.IsNaN(linacc.X), test.ShouldBeTrue)
	test.That(t, math.IsNaN(linacc.Y), test.ShouldBeTrue)
	test.That(t, math.IsNaN(linacc.Z), test.ShouldBeTrue)

	ori, err := ms.Orientation(ctx, nil)
	test.That(t, err, test.ShouldBeError, movementsensor.ErrMethodUnimplementedOrientation)
	test.That(t, math.IsNaN(ori.OrientationVectorRadians().OX), test.ShouldBeTrue)
	test.That(t, math.IsNaN(ori.OrientationVectorRadians().OY), test.ShouldBeTrue)
	test.That(t, math.IsNaN(ori.OrientationVectorRadians().OZ), test.ShouldBeTrue)
	test.That(t, math.IsNaN(ori.OrientationVectorRadians().Theta), test.ShouldBeTrue)

	linvel, err := ms.LinearVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, linvel, test.ShouldResemble, testlinvel)

	angvel, err := ms.AngularVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angvel, test.ShouldResemble, testangvel)

	linacc, err = ms.LinearAcceleration(ctx, nil)
	test.That(t, err, test.ShouldBeError, movementsensor.ErrMethodUnimplementedLinearAcceleration)
	test.That(t, math.IsNaN(linacc.X), test.ShouldBeTrue)
	test.That(t, math.IsNaN(linacc.Y), test.ShouldBeTrue)
	test.That(t, math.IsNaN(linacc.Z), test.ShouldBeTrue)

	properties, err := ms.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, properties.OrientationSupported, test.ShouldBeFalse)
	test.That(t, properties.PositionSupported, test.ShouldBeFalse)
	test.That(t, properties.CompassHeadingSupported, test.ShouldBeFalse)
	test.That(t, properties.LinearAccelerationSupported, test.ShouldBeFalse)
	test.That(t, properties.AngularVelocitySupported, test.ShouldBeTrue)
	test.That(t, properties.LinearVelocitySupported, test.ShouldBeTrue)

	//nolint:dogsled
	accuracies, _, _, _, _, err := ms.Accuracy(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, accuracies, test.ShouldResemble,
		map[string]float32{
			"goodAngVel_accuracy": 32,
			"goodLinVel_accuracy": 32,
		})

	readings, err := ms.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings, test.ShouldResemble, map[string]interface{}{
		"linear_velocity":  linvel,
		"angular_velocity": angvel,
	})

	conf = setUpCfg(oriSensors, posSensors, compassSensors, linvelSensors, angvelSensors, linaccSensors)

	depmap[linvelSensors[0]] = linvelProps
	depmap[linvelSensors[1]] = linvelProps
	depmap[angvelSensors[0]] = angvelProps
	depmap[angvelSensors[1]] = angvelProps
	depmap[oriSensors[0]] = oriProps
	depmap[oriSensors[1]] = angvelProps // wrong properties for first compass sensor
	depmap[posSensors[0]] = posProps
	depmap[posSensors[1]] = emptyProps
	depmap[compassSensors[0]] = emptyProps
	depmap[compassSensors[1]] = compassProps
	depmap[compassSensors[2]] = emptyProps
	depmap[linaccSensors[0]] = linaccProps
	depmap[linaccSensors[1]] = emptyProps

	deps = setupDependencies(t, depmap, false, false)

	// first reconfiguration with six sensors and no function errors
	err = ms.Reconfigure(ctx, deps, conf)
	test.That(t, err, test.ShouldBeNil)

	res := ms.Name()
	test.That(t, res, test.ShouldNotBeNil)

	pos, alt, err = ms.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, testgeopoint)
	test.That(t, alt, test.ShouldEqual, testalt)

	ori, err = ms.Orientation(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ori, test.ShouldEqual, testori)

	compass, err = ms.CompassHeading(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, compass, test.ShouldEqual, testcompass)

	linvel, err = ms.LinearVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, linvel, test.ShouldResemble, testlinvel)

	angvel, err = ms.AngularVelocity(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angvel, test.ShouldResemble, testangvel)

	linacc, err = ms.LinearAcceleration(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, linacc, test.ShouldResemble, testlinacc)

	properties, err = ms.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, properties.OrientationSupported, test.ShouldBeTrue)
	test.That(t, properties.PositionSupported, test.ShouldBeTrue)
	test.That(t, properties.CompassHeadingSupported, test.ShouldBeTrue)
	test.That(t, properties.LinearAccelerationSupported, test.ShouldBeTrue)
	test.That(t, properties.AngularVelocitySupported, test.ShouldBeTrue)
	test.That(t, properties.LinearVelocitySupported, test.ShouldBeTrue)

	//nolint:dogsled
	accuracies, _, _, _, _, err = ms.Accuracy(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, accuracies, test.ShouldResemble,
		map[string]float32{
			"goodOri_accuracy":     32,
			"goodPos_accuracy":     32,
			"goodCompass_accuracy": 32,
			"goodAngVel_accuracy":  32,
			"goodLinVel_accuracy":  32,
			"goodLinAcc_accuracy":  32,
		})

	readings, err = ms.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings, test.ShouldResemble, map[string]interface{}{
		"orientation":         ori,
		"position":            pos,
		"altitude":            alt,
		"compass":             compass,
		"linear_velocity":     linvel,
		"angular_velocity":    angvel,
		"linear_acceleration": linacc,
	})

	// second reconfiguration with six sensors but an error in accuracy
	deps = setupDependencies(t, depmap, true /* accuracy error */, false)
	err = ms.Reconfigure(ctx, deps, conf)
	test.That(t, err, test.ShouldBeNil)

	//nolint:dogsled
	accuracies, _, _, _, _, err = ms.Accuracy(ctx, nil)
	test.That(t, err, test.ShouldNotBeNil)

	for k, v := range accuracies {
		test.That(t, k, test.ShouldContainSubstring, errStrAccuracy)
		test.That(t, math.IsNaN(float64(v)), test.ShouldBeTrue)
	}

	// close the sensor, this test is done
	test.That(t, ms.Close(ctx), test.ShouldBeNil)
}
