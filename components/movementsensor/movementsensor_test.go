package movementsensor_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testMovementSensorName    = "gps1"
	testMovementSensorName2   = "gps2"
	failMovementSensorName    = "gps3"
	fakeMovementSensorName    = "gps4"
	missingMovementSensorName = "gps5"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	deps[movementsensor.Named(testMovementSensorName)] = &mock{Name: testMovementSensorName}
	deps[movementsensor.Named(fakeMovementSensorName)] = "not an gps"
	return deps
}

func setupInjectRobot() *inject.Robot {
	gps1 := &mock{Name: testMovementSensorName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case movementsensor.Named(testMovementSensorName):
			return gps1, nil
		case movementsensor.Named(fakeMovementSensorName):
			return "not a gps", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{movementsensor.Named(testMovementSensorName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	g, err := movementsensor.FromRobot(r, testMovementSensorName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := g.DoCommand(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	s, err := movementsensor.FromDependencies(deps, testMovementSensorName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	result, _, err := s.Position(context.Background(), make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, loc)

	s, err = movementsensor.FromDependencies(deps, fakeMovementSensorName)
	test.That(t, err, test.ShouldBeError, movementsensor.DependencyTypeError(fakeMovementSensorName, "string"))
	test.That(t, s, test.ShouldBeNil)

	s, err = movementsensor.FromDependencies(deps, missingMovementSensorName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyNotFoundError(missingMovementSensorName))
	test.That(t, s, test.ShouldBeNil)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	s, err := movementsensor.FromRobot(r, testMovementSensorName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	result, _, err := s.Position(context.Background(), make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, loc)

	s, err = movementsensor.FromRobot(r, fakeMovementSensorName)
	test.That(t, err, test.ShouldBeError, movementsensor.NewUnimplementedInterfaceError("string"))
	test.That(t, s, test.ShouldBeNil)

	s, err = movementsensor.FromRobot(r, missingMovementSensorName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(movementsensor.Named(missingMovementSensorName)))
	test.That(t, s, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := movementsensor.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testMovementSensorName})
}

func TestMovementSensorName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: movementsensor.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testMovementSensorName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: movementsensor.SubtypeName,
				},
				Name: testMovementSensorName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := movementsensor.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualMovementSensor1 movementsensor.MovementSensor = &mock{Name: testMovementSensorName}
	reconfMovementSensor1, err := movementsensor.WrapWithReconfigurable(actualMovementSensor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = movementsensor.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, movementsensor.NewUnimplementedInterfaceError(nil))

	reconfMovementSensor2, err := movementsensor.WrapWithReconfigurable(reconfMovementSensor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMovementSensor2, test.ShouldEqual, reconfMovementSensor1)
}

func TestReconfigurableMovementSensor(t *testing.T) {
	actualMovementSensor1 := &mock{Name: testMovementSensorName}
	reconfMovementSensor1, err := movementsensor.WrapWithReconfigurable(actualMovementSensor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	actualMovementSensor2 := &mock{Name: testMovementSensorName2}
	reconfMovementSensor2, err := movementsensor.WrapWithReconfigurable(actualMovementSensor2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMovementSensor1.reconfCount, test.ShouldEqual, 0)

	err = reconfMovementSensor1.Reconfigure(context.Background(), reconfMovementSensor2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMovementSensor1, test.ShouldResemble, reconfMovementSensor2)
	test.That(t, actualMovementSensor1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualMovementSensor1.positionCount, test.ShouldEqual, 0)
	test.That(t, actualMovementSensor2.positionCount, test.ShouldEqual, 0)
	result, _, err := reconfMovementSensor1.(movementsensor.MovementSensor).Position(context.Background(), make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, loc)
	test.That(t, actualMovementSensor1.positionCount, test.ShouldEqual, 0)
	test.That(t, actualMovementSensor2.positionCount, test.ShouldEqual, 1)

	err = reconfMovementSensor1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfMovementSensor1, nil))

	actualMovementSensor3 := &mock{Name: failMovementSensorName}
	reconfMovementSensor3, err := movementsensor.WrapWithReconfigurable(actualMovementSensor3, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMovementSensor3, test.ShouldNotBeNil)
}

func TestPosition(t *testing.T) {
	actualMovementSensor1 := &mock{Name: testMovementSensorName}
	reconfMovementSensor1, _ := movementsensor.WrapWithReconfigurable(actualMovementSensor1, resource.Name{})

	test.That(t, actualMovementSensor1.positionCount, test.ShouldEqual, 0)
	loc1, _, err := reconfMovementSensor1.(movementsensor.MovementSensor).Position(context.Background(), make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldResemble, geo.NewPoint(90, 1))
	test.That(t, actualMovementSensor1.positionCount, test.ShouldEqual, 1)
}

func TestLinearVelocity(t *testing.T) {
	actualMovementSensor1 := &mock{Name: testMovementSensorName}
	reconfMovementSensor1, _ := movementsensor.WrapWithReconfigurable(actualMovementSensor1, resource.Name{})

	test.That(t, actualMovementSensor1.velocityCount, test.ShouldEqual, 0)
	speed1, err := reconfMovementSensor1.(movementsensor.MovementSensor).LinearVelocity(context.Background(), make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1, test.ShouldResemble, speed)
	test.That(t, actualMovementSensor1.velocityCount, test.ShouldEqual, 1)
}

func TestReadings(t *testing.T) {
	actualMovementSensor1 := &mock{Name: testMovementSensorName}
	reconfMovementSensor1, _ := movementsensor.WrapWithReconfigurable(actualMovementSensor1, resource.Name{})

	readings1, err := movementsensor.Readings(context.Background(), actualMovementSensor1, make(map[string]interface{}))
	allReadings := map[string]interface{}{
		"altitude":            alt,
		"angular_velocity":    ang,
		"compass":             compass,
		"linear_velocity":     speed,
		"orientation":         orie,
		"position":            loc,
		"linear_acceleration": la,
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings1, test.ShouldResemble, allReadings)

	result, err := reconfMovementSensor1.(sensor.Sensor).Readings(context.Background(), make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, readings1)

	p, err := protoutils.ReadingGoToProto(allReadings)
	test.That(t, err, test.ShouldBeNil)
	r2, err := protoutils.ReadingProtoToGo(p)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r2, test.ShouldResemble, allReadings)
}

func TestClose(t *testing.T) {
	actualMovementSensor1 := &mock{Name: testMovementSensorName}
	reconfMovementSensor1, _ := movementsensor.WrapWithReconfigurable(actualMovementSensor1, resource.Name{})

	test.That(t, actualMovementSensor1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfMovementSensor1), test.ShouldBeNil)
	test.That(t, actualMovementSensor1.reconfCount, test.ShouldEqual, 1)
}

var (
	loc     = geo.NewPoint(90, 1)
	alt     = 50.5
	speed   = r3.Vector{5.4, 1.1, 2.2}
	ang     = spatialmath.AngularVelocity{5.5, 1.2, 2.3}
	orie    = &spatialmath.EulerAngles{5.6, 1.3, 2.4}
	compass = 123.
	la      = r3.Vector{1.1, 5.6, 0}
)

type mock struct {
	movementsensor.MovementSensor
	Name          string
	reconfCount   int
	positionCount int
	velocityCount int
}

func (m *mock) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func (m *mock) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	m.positionCount++
	return loc, alt, nil
}

func (m *mock) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	m.velocityCount++
	return speed, nil
}

func (m *mock) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return la, nil
}

func (m *mock) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return ang, nil
}

func (m *mock) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return orie, nil
}

func (m *mock) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return compass, nil
}

func (m *mock) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return movementsensor.Readings(ctx, m, extra)
}

func (m *mock) Close() { m.reconfCount++ }
