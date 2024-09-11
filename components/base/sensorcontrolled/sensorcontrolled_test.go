package sensorcontrolled

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	// compassValue and orientationValue should be different for tests.
	defaultCompassValue     = 45.
	defaultOrientationValue = 40.
	wrongTypeLinVel         = "linear"
	wrongTypeAngVel         = "angulr_velocity"
)

var (
	// compassValue and orientationValue should be different for tests.
	compassValue     = defaultCompassValue
	orientationValue = defaultOrientationValue
)

func sConfig() resource.Config {
	return resource.Config{
		Name:  "test",
		API:   base.API,
		Model: resource.Model{Name: "wheeled_base"},
		ConvertedAttributes: &Config{
			MovementSensor: []string{"ms"},
			Base:           "test_base",
		},
	}
}

func createDependencies(t *testing.T) resource.Dependencies {
	t.Helper()
	deps := make(resource.Dependencies)

	counter := 0

	deps[movementsensor.Named("ms")] = &inject.MovementSensor{
		PropertiesFuncExtraCap: map[string]interface{}{},
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return &movementsensor.Properties{OrientationSupported: true}, nil
		},
		OrientationFunc: func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
			counter++
			return &spatialmath.EulerAngles{Roll: 0, Pitch: 0, Yaw: rdkutils.RadToDeg(float64(counter))}, nil
		},
	}

	deps = addBaseDependency(deps)

	return deps
}

func addBaseDependency(deps resource.Dependencies) resource.Dependencies {
	deps[base.Named(("test_base"))] = &inject.Base{
		DoFunc: testutils.EchoFunc,
		MoveStraightFunc: func(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
			return nil
		},
		SpinFunc: func(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
			return nil
		},
		StopFunc: func(ctx context.Context, extra map[string]interface{}) error {
			return nil
		},
		IsMovingFunc: func(context.Context) (bool, error) {
			return false, nil
		},
		CloseFunc: func(ctx context.Context) error {
			return nil
		},
		SetPowerFunc: func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
			return nil
		},
		SetVelocityFunc: func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
			return nil
		},
		PropertiesFunc: func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
			return base.Properties{
				TurningRadiusMeters: 0.1,
				WidthMeters:         0.1,
			}, nil
		},
		GeometriesFunc: func(ctx context.Context) ([]spatialmath.Geometry, error) {
			return nil, nil
		},
	}
	return deps
}

func TestSensorBase(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	testCfg := sConfig()
	conf, ok := testCfg.ConvertedAttributes.(*Config)
	test.That(t, ok, test.ShouldBeTrue)
	deps, err := conf.Validate("path")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldResemble, []string{"ms", "test_base"})
	sbDeps := createDependencies(t)

	sb, err := createSensorBase(ctx, sbDeps, testCfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb, test.ShouldNotBeNil)

	moving, err := sb.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, moving, test.ShouldBeFalse)

	props, err := sb.Properties(ctx, nil)
	test.That(t, props.WidthMeters, test.ShouldResemble, 0.1)
	test.That(t, err, test.ShouldBeNil)

	geometries, err := sb.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, geometries, test.ShouldBeNil)

	test.That(t, sb.SetPower(ctx, r3.Vector{X: 0, Y: 10, Z: 0}, r3.Vector{X: 0, Y: 0, Z: 0}, nil), test.ShouldBeNil)

	// this test does not include a velocities sensor and does not create a sensor base with a control loop
	test.That(t, sb.SetVelocity(ctx, r3.Vector{X: 0, Y: 100, Z: 0}, r3.Vector{X: 0, Y: 100, Z: 0}, nil), test.ShouldBeNil)
	test.That(t, sb.MoveStraight(ctx, 10, 10, nil), test.ShouldBeNil)
	test.That(t, sb.Spin(ctx, 2, 10, nil), test.ShouldBeNil)
	test.That(t, sb.Stop(ctx, nil), test.ShouldBeNil)

	test.That(t, sb.Close(ctx), test.ShouldBeNil)
}

func sBaseTestConfig(msNames []string, freq float64, linType, angType string) resource.Config {
	controlParams := make([]control.PIDConfig, 2)
	controlParams[0] = control.PIDConfig{
		Type: linType,
		P:    0.5,
		I:    0.5,
		D:    0.0,
	}
	controlParams[1] = control.PIDConfig{
		Type: angType,
		P:    0.5,
		I:    0.5,
		D:    0.0,
	}

	return resource.Config{
		Name:  "test",
		API:   base.API,
		Model: resource.Model{Name: "controlled_base"},
		ConvertedAttributes: &Config{
			MovementSensor:    msNames,
			Base:              "test_base",
			ControlParameters: controlParams,
			ControlFreq:       freq,
		},
	}
}

func msDependencies(t *testing.T, msNames []string,
) (resource.Dependencies, resource.Config) {
	t.Helper()

	cfg := sBaseTestConfig(msNames, defaultControlFreq, typeLinVel, typeAngVel)

	deps := make(resource.Dependencies)

	for _, msName := range msNames {
		ms := inject.NewMovementSensor(msName)
		switch {
		case strings.Contains(msName, "orientation"):
			ms.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
				return &movementsensor.Properties{
					OrientationSupported: true,
				}, nil
			}
			ms.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
				return &spatialmath.EulerAngles{Roll: 0, Pitch: 0, Yaw: rdkutils.DegToRad(orientationValue)}, nil
			}
			deps[movementsensor.Named(msName)] = ms
		case strings.Contains(msName, "position"):
			ms.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
				return &movementsensor.Properties{
					PositionSupported: true,
				}, nil
			}
			ms.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
				return &geo.Point{}, 0, nil
			}
			deps[movementsensor.Named(msName)] = ms
		case strings.Contains(msName, "compass"):
			ms.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
				return &movementsensor.Properties{
					CompassHeadingSupported: true,
				}, nil
			}
			ms.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
				return compassValue, nil
			}
			deps[movementsensor.Named(msName)] = ms

		case strings.Contains(msName, "setvel"):
			ms.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
				return &movementsensor.Properties{
					AngularVelocitySupported: true,
					LinearVelocitySupported:  true,
				}, nil
			}
			ms.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
				return r3.Vector{}, nil
			}
			ms.AngularVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
				return spatialmath.AngularVelocity{}, nil
			}
			deps[movementsensor.Named(msName)] = ms

		case strings.Contains(msName, "Bad"):
			ms.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
				return &movementsensor.Properties{
					OrientationSupported:     true,
					AngularVelocitySupported: true,
					LinearVelocitySupported:  true,
				}, errors.New("bad sensor")
			}
			deps[movementsensor.Named(msName)] = ms

		default:
		}
	}

	deps = addBaseDependency(deps)

	return deps, cfg
}

func TestReconfig(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	deps, cfg := msDependencies(t, []string{"orientation"})

	b, err := createSensorBase(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	sb, ok := b.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)
	headingOri, headingSupported, err := sb.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeTrue)
	test.That(t, headingOri, test.ShouldEqual, orientationValue)
	test.That(t, sb.controlFreq, test.ShouldEqual, defaultControlFreq)

	deps, cfg = msDependencies(t, []string{"orientation1"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeTrue)
	test.That(t, headingOri, test.ShouldEqual, orientationValue)

	deps, cfg = msDependencies(t, []string{"setvel1"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel1")

	deps, _ = msDependencies(t, []string{"setvel2"})
	// generate a config with a non default freq
	cfg = sBaseTestConfig([]string{"setvel2"}, 100, typeLinVel, typeAngVel)
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel2")
	headingNone, headingSupported, err := sb.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeFalse)
	test.That(t, headingNone, test.ShouldEqual, 0)
	test.That(t, sb.controlFreq, test.ShouldEqual, 100.0)

	deps, cfg = msDependencies(t, []string{"orientation3", "setvel3", "Bad"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	headingOri, headingSupported, err = sb.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeTrue)
	test.That(t, headingOri, test.ShouldEqual, orientationValue)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel3")

	deps, cfg = msDependencies(t, []string{"Bad", "orientation4", "setvel4", "orientation5", "setvel5"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	headingOri, headingSupported, err = sb.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeTrue)
	test.That(t, headingOri, test.ShouldEqual, orientationValue)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel4")

	deps, cfg = msDependencies(t, []string{"Bad", "orientation6", "setvel6", "position1", "compass1"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	headingOri, headingSupported, err = sb.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeTrue)
	test.That(t, headingOri, test.ShouldEqual, orientationValue)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel6")
	test.That(t, sb.position.Name().ShortName(), test.ShouldResemble, "position1")

	deps, cfg = msDependencies(t, []string{"Bad", "setvel7", "position2", "compass2"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel7")
	test.That(t, sb.position.Name().ShortName(), test.ShouldResemble, "position2")
	headingCompass, headingSupported, err := sb.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeTrue)
	test.That(t, headingCompass, test.ShouldNotEqual, orientationValue)
	test.That(t, headingCompass, test.ShouldEqual, -compassValue)

	deps, cfg = msDependencies(t, []string{"Bad"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, sb.velocities, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError, errNoGoodSensor)
	headingBad, headingSupported, err := sb.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeFalse)
	test.That(t, headingBad, test.ShouldEqual, 0)

	deps, _ = msDependencies(t, []string{"setvel2"})
	// generate a config with invalid pid types
	cfg = sBaseTestConfig([]string{"setvel2"}, 100, wrongTypeLinVel, wrongTypeAngVel)
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err.Error(), test.ShouldContainSubstring, "type must be 'linear_velocity' or 'angular_velocity'")
}

func TestSensorBaseWithVelocitiesSensor(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	deps, _ := msDependencies(t, []string{"setvel1"})
	// generate a config with a non default freq
	cfg := sBaseTestConfig([]string{"setvel1"}, 100, typeLinVel, typeAngVel)

	b, err := createSensorBase(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	sb, ok := b.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel1")

	test.That(t, sb.SetVelocity(ctx, r3.Vector{X: 0, Y: 100, Z: 0}, r3.Vector{X: 0, Y: 100, Z: 0}, nil), test.ShouldBeNil)
	test.That(t, sb.loop, test.ShouldNotBeNil)
	loopFreq, err := sb.loop.Frequency(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loopFreq, test.ShouldEqual, 100)
	test.That(t, sb.Stop(ctx, nil), test.ShouldBeNil)
	test.That(t, sb.Close(ctx), test.ShouldBeNil)
}

func TestSensorBaseSpin(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	deps, cfg := msDependencies(t, []string{"setvel1", "orientation1"})
	b, err := createSensorBase(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	sb, ok := b.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	headingOri, headingSupported, err := sb.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeTrue)
	test.That(t, headingOri, test.ShouldEqual, orientationValue)

	depsNoOri, cfgNoOri := msDependencies(t, []string{"setvel1"})
	bNoOri, err := createSensorBase(ctx, depsNoOri, cfgNoOri, logger)
	test.That(t, err, test.ShouldBeNil)
	sbNoOri, ok := bNoOri.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	headingOri, headingSupported, err = sbNoOri.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeFalse)
	test.That(t, headingOri, test.ShouldEqual, 0)
	t.Run("Test canceling a sensor controlled spin", func(t *testing.T) {
		// flaky test, will see behavior after RSDK-6164
		t.Skip()
		cancelCtx, cancel := context.WithCancel(ctx)
		wg := sync.WaitGroup{}
		wg.Add(1)
		utils.PanicCapturingGo(func() {
			defer wg.Done()
			err := sb.Spin(cancelCtx, 10, 10, nil)
			test.That(t, err, test.ShouldBeError, cancelCtx.Err())
		})
		time.Sleep(4 * time.Second)
		cancel()
		wg.Wait()
	})
	t.Run("Test canceling a sensor controlled spin due to calling another running api", func(t *testing.T) {
		// flaky test, will see behavior after RSDK-6164
		t.Skip()
		wg := sync.WaitGroup{}
		wg.Add(1)
		utils.PanicCapturingGo(func() {
			defer wg.Done()
			err := sb.Spin(ctx, 10, 10, nil)
			test.That(t, err, test.ShouldBeNil)
		})
		time.Sleep(2 * time.Second)
		err := sb.SetPower(context.Background(), r3.Vector{}, r3.Vector{}, nil)
		test.That(t, err, test.ShouldBeNil)
		wg.Wait()
	})
	t.Run("Test not including an orientation ms will use the sensor controlled spin", func(t *testing.T) {
		// flaky test, will see behavior after RSDK-6164
		t.Skip()
		err := sbNoOri.Spin(ctx, 10, 10, nil)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestSensorBaseMoveStraight(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	deps, cfg := msDependencies(t, []string{"setvel1", "position1", "orientation1"})
	b, err := createSensorBase(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	sb, ok := b.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.position.Name().ShortName(), test.ShouldResemble, "position1")
	headingOri, headingSupported, err := sb.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeTrue)
	test.That(t, headingOri, test.ShouldEqual, orientationValue)
	test.That(t, headingOri, test.ShouldNotEqual, compassValue)

	depsNoPos, cfgNoPos := msDependencies(t, []string{"setvel1"})
	bNoPos, err := createSensorBase(ctx, depsNoPos, cfgNoPos, logger)
	test.That(t, err, test.ShouldBeNil)
	sbNoPos, ok := bNoPos.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	headingZero, headingSupported, err := sbNoPos.headingFunc(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, headingSupported, test.ShouldBeFalse)
	test.That(t, headingZero, test.ShouldEqual, 0)
	t.Run("Test canceling a sensor controlled MoveStraight", func(t *testing.T) {
		// flaky test, will see behavior after RSDK-6164
		t.Skip()
		cancelCtx, cancel := context.WithCancel(ctx)
		wg := sync.WaitGroup{}
		wg.Add(1)
		utils.PanicCapturingGo(func() {
			defer wg.Done()
			err := sb.MoveStraight(cancelCtx, 100, 100, nil)
			test.That(t, err, test.ShouldBeNil)
		})
		time.Sleep(4 * time.Second)
		cancel()
		wg.Wait()
	})
	t.Run("Test canceling a sensor controlled MoveStraight due to calling another running api", func(t *testing.T) {
		// flaky test, will see behavior after RSDK-6164
		t.Skip()
		wg := sync.WaitGroup{}
		wg.Add(1)
		utils.PanicCapturingGo(func() {
			defer wg.Done()
			err := sb.MoveStraight(ctx, 100, 100, nil)
			test.That(t, err, test.ShouldBeNil)
		})
		time.Sleep(2 * time.Second)
		err := sb.SetPower(context.Background(), r3.Vector{}, r3.Vector{}, nil)
		test.That(t, err, test.ShouldBeNil)
		wg.Wait()
	})
	t.Run("Test not including a position ms will use the controlled MoveStraight", func(t *testing.T) {
		// flaky test, will see behavior after RSDK-6164
		t.Skip()
		err := sbNoPos.MoveStraight(ctx, 100, 100, nil)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("Test heading error wraps", func(t *testing.T) {
		// orientation configured, so update the value for testing
		orientationValue = 179
		headingOri, headingSupported, err := sb.headingFunc(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, headingSupported, test.ShouldBeTrue)
		// validate the orientation updated
		test.That(t, headingOri, test.ShouldEqual, 179)

		// test -179 -> 179 results in a small error
		headingErr, err := sb.calcHeadingControl(ctx, -179)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, headingErr, test.ShouldEqual, 2)

		// test full circle results in 0 error
		headingErr2, err := sb.calcHeadingControl(ctx, 360+179)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, headingErr2, test.ShouldEqual, 0)
		for i := -720; i <= 720; i += 30 {
			headingErr, err := sb.calcHeadingControl(ctx, float64(i))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, headingErr, test.ShouldBeBetweenOrEqual, -180, 180)
		}
		orientationValue = defaultOrientationValue
	})
}

func TestSensorBaseDoCommand(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	deps, cfg := msDependencies(t, []string{"setvel1", "position1", "orientation1"})
	b, err := createSensorBase(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	sb, ok := b.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)

	expectedPID := control.PIDConfig{P: 0.1, I: 2.0, D: 0.0}
	sb.tunedVals = &[]control.PIDConfig{expectedPID, {}}
	expectedeMap := make(map[string]interface{})
	expectedeMap["get_tuned_pid"] = (fmt.Sprintf("{p: %v, i: %v, d: %v, type: %v} ",
		expectedPID.P, expectedPID.I, expectedPID.D, expectedPID.Type))

	req := make(map[string]interface{})
	req["get_tuned_pid"] = true
	resp, err := b.DoCommand(ctx, req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, expectedeMap)

	emptyMap := make(map[string]interface{})
	req["get_tuned_pid"] = false
	resp, err = b.DoCommand(ctx, req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, emptyMap)
}
