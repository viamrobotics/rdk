package sensorcontrolled

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
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

func sBaseTestConfig(msNames []string) resource.Config {
	controlParams := make([]control.PIDConfig, 2)
	controlParams[0] = control.PIDConfig{
		Type: typeLinVel,
		P:    0.5,
		I:    0.5,
		D:    0.0,
	}
	controlParams[1] = control.PIDConfig{
		Type: typeAngVel,
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
		},
	}
}

func msDependencies(t *testing.T, msNames []string,
) (resource.Dependencies, resource.Config) {
	t.Helper()

	cfg := sBaseTestConfig(msNames)

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
				return &spatialmath.EulerAngles{Roll: 0, Pitch: 0, Yaw: 5}, nil
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
	test.That(t, sb.orientation.Name().ShortName(), test.ShouldResemble, "orientation")

	deps, cfg = msDependencies(t, []string{"orientation1"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.orientation.Name().ShortName(), test.ShouldResemble, "orientation1")

	deps, cfg = msDependencies(t, []string{"orientation2"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.orientation.Name().ShortName(), test.ShouldResemble, "orientation2")

	deps, cfg = msDependencies(t, []string{"setvel1"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel1")

	deps, cfg = msDependencies(t, []string{"setvel2"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel2")

	deps, cfg = msDependencies(t, []string{"orientation3", "setvel3", "Bad"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.orientation.Name().ShortName(), test.ShouldResemble, "orientation3")
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel3")

	deps, cfg = msDependencies(t, []string{"Bad", "orientation4", "setvel4", "orientation5", "setvel5"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.orientation.Name().ShortName(), test.ShouldResemble, "orientation4")
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel4")

	deps, cfg = msDependencies(t, []string{"Bad"})
	err = b.Reconfigure(ctx, deps, cfg)
	test.That(t, sb.orientation, test.ShouldBeNil)
	test.That(t, sb.velocities, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError, errNoGoodSensor)
}

func TestSensorBaseWithVelocitiesSensor(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	deps, cfg := msDependencies(t, []string{"setvel1"})

	b, err := createSensorBase(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	sb, ok := b.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.velocities.Name().ShortName(), test.ShouldResemble, "setvel1")

	test.That(t, sb.SetVelocity(ctx, r3.Vector{X: 0, Y: 100, Z: 0}, r3.Vector{X: 0, Y: 100, Z: 0}, nil), test.ShouldBeNil)
	test.That(t, sb.loop, test.ShouldNotBeNil)
	test.That(t, sb.Stop(ctx, nil), test.ShouldBeNil)
}

func TestSensorBaseSpin(t *testing.T) {
	// flaky test, will see behavior after RSDK-6164
	t.Skip()
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	deps, cfg := msDependencies(t, []string{"setvel1", "orientation1"})
	b, err := createSensorBase(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	sb, ok := b.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sb.orientation.Name().ShortName(), test.ShouldResemble, "orientation1")

	depsNoOri, cfgNoOri := msDependencies(t, []string{"setvel1"})
	bNoOri, err := createSensorBase(ctx, depsNoOri, cfgNoOri, logger)
	test.That(t, err, test.ShouldBeNil)
	sbNoOri, ok := bNoOri.(*sensorBase)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sbNoOri.orientation, test.ShouldBeNil)
	t.Run("Test canceling a sensor controlled spin", func(t *testing.T) {
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
	t.Run("Test not including an orientation ms will use the non controlled spin", func(t *testing.T) {
		// the injected base will return nil instead of blocking
		err := sbNoOri.Spin(ctx, 10, 10, nil)
		test.That(t, err, test.ShouldBeNil)
	})
}
