package imu_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testIMUName    = "imu1"
	testIMUName2   = "imu2"
	fakeIMUName    = "imu3"
	missingIMUName = "imu4"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	deps[imu.Named(testIMUName)] = &mock{Name: testIMUName}
	deps[imu.Named(fakeIMUName)] = "not an imu"
	return deps
}

func setupInjectRobot() *inject.Robot {
	imu1 := &mock{Name: testIMUName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case imu.Named(testIMUName):
			return imu1, nil
		case imu.Named(fakeIMUName):
			return "not an imu", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{imu.Named(testIMUName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	i, err := imu.FromRobot(r, testIMUName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, i, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := i.Do(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	s, err := imu.FromDependencies(deps, testIMUName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	result, err := s.ReadOrientation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, ea)

	s, err = imu.FromDependencies(deps, fakeIMUName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyTypeError(fakeIMUName, "IMU", "string"))
	test.That(t, s, test.ShouldBeNil)

	s, err = imu.FromDependencies(deps, missingIMUName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyNotFoundError(missingIMUName))
	test.That(t, s, test.ShouldBeNil)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	s, err := imu.FromRobot(r, testIMUName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	result, err := s.ReadOrientation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, ea)

	s, err = imu.FromRobot(r, fakeIMUName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("IMU", "string"))
	test.That(t, s, test.ShouldBeNil)

	s, err = imu.FromRobot(r, missingIMUName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(imu.Named(missingIMUName)))
	test.That(t, s, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := imu.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testIMUName})
}

func TestIMUName(t *testing.T) {
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
					ResourceSubtype: imu.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testIMUName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: imu.SubtypeName,
				},
				Name: testIMUName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := imu.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

var (
	av = spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3}
	ea = &spatialmath.EulerAngles{Roll: 4, Pitch: 5, Yaw: 6}
	ac = r3.Vector{X: 7, Y: 8, Z: 9}
	mg = r3.Vector{X: 10, Y: 11, Z: 12}

	readings = []interface{}{5.6, 6.4}
)

func TestWrapWithReconfigurable(t *testing.T) {
	var actualIMU1 imu.IMU = &mock{Name: testIMUName}
	reconfIMU1, err := imu.WrapWithReconfigurable(actualIMU1)
	test.That(t, err, test.ShouldBeNil)

	_, err = imu.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("IMU", nil))

	reconfIMU2, err := imu.WrapWithReconfigurable(reconfIMU1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfIMU2, test.ShouldEqual, reconfIMU1)
}

func TestReconfigurableIMU(t *testing.T) {
	actualIMU1 := &mock{Name: testIMUName}
	reconfIMU1, err := imu.WrapWithReconfigurable(actualIMU1)
	test.That(t, err, test.ShouldBeNil)

	actualIMU2 := &mock{Name: testIMUName2}
	reconfIMU2, err := imu.WrapWithReconfigurable(actualIMU2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualIMU1.reconfCount, test.ShouldEqual, 0)

	err = reconfIMU1.Reconfigure(context.Background(), reconfIMU2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfIMU1, test.ShouldResemble, reconfIMU2)
	test.That(t, actualIMU1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualIMU1.orientationCount, test.ShouldEqual, 0)
	test.That(t, actualIMU2.orientationCount, test.ShouldEqual, 0)
	result, err := reconfIMU1.(imu.IMU).ReadOrientation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, ea)
	test.That(t, actualIMU1.orientationCount, test.ShouldEqual, 0)
	test.That(t, actualIMU2.orientationCount, test.ShouldEqual, 1)

	err = reconfIMU1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *imu.reconfigurableIMU")
}

func TestReadAngularVelocity(t *testing.T) {
	actualIMU1 := &mock{Name: testIMUName}
	reconfIMU1, _ := imu.WrapWithReconfigurable(actualIMU1)

	test.That(t, actualIMU1.angularVelocityCount, test.ShouldEqual, 0)
	vel, err := reconfIMU1.(imu.IMU).ReadAngularVelocity(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vel, test.ShouldResemble, spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3})
	test.That(t, actualIMU1.angularVelocityCount, test.ShouldEqual, 1)
}

func TestReadOrientation(t *testing.T) {
	actualIMU1 := &mock{Name: testIMUName}
	reconfIMU1, _ := imu.WrapWithReconfigurable(actualIMU1)

	test.That(t, actualIMU1.orientationCount, test.ShouldEqual, 0)
	angles, err := reconfIMU1.(imu.IMU).ReadOrientation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angles, test.ShouldResemble, &spatialmath.EulerAngles{Roll: 4, Pitch: 5, Yaw: 6})
	test.That(t, actualIMU1.orientationCount, test.ShouldEqual, 1)
}

func TestReadAcceleration(t *testing.T) {
	actualIMU1 := &mock{Name: testIMUName}
	reconfIMU1, _ := imu.WrapWithReconfigurable(actualIMU1)

	test.That(t, actualIMU1.accelerationCount, test.ShouldEqual, 0)
	acc, err := reconfIMU1.(imu.IMU).ReadAcceleration(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, acc, test.ShouldResemble, r3.Vector{X: 7, Y: 8, Z: 9})
	test.That(t, actualIMU1.accelerationCount, test.ShouldEqual, 1)
}

func TestReadMagnetometer(t *testing.T) {
	actualIMU1 := &mock{Name: testIMUName}
	reconfIMU1, _ := imu.WrapWithReconfigurable(actualIMU1)

	test.That(t, actualIMU1.magnetometerCount, test.ShouldEqual, 0)
	mag, err := reconfIMU1.(imu.IMU).ReadMagnetometer(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mag, test.ShouldResemble, r3.Vector{X: 10, Y: 11, Z: 12})
	test.That(t, actualIMU1.magnetometerCount, test.ShouldEqual, 1)
}

func TestGetReadings(t *testing.T) {
	actualIMU1 := &mock{Name: testIMUName}
	reconfIMU1, _ := imu.WrapWithReconfigurable(actualIMU1)

	readings1, err := imu.GetReadings(context.Background(), actualIMU1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings1, test.ShouldResemble, []interface{}{
		av.X, av.Y, av.Z,
		ea.Roll, ea.Pitch, ea.Yaw,
		ac.X, ac.Y, ac.Z,
		mg.X, mg.Y, mg.Z,
	})

	result, err := reconfIMU1.(sensor.Sensor).GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, readings1)

	actualIMU2 := &mockWithSensor{}
	reconfIMU2, _ := imu.WrapWithReconfigurable(actualIMU2)

	test.That(t, actualIMU2.readingsCount, test.ShouldEqual, 0)
	result, err = reconfIMU2.(sensor.Sensor).GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, readings)
	test.That(t, actualIMU2.readingsCount, test.ShouldEqual, 1)
}

type mock struct {
	imu.IMU
	Name                 string
	angularVelocityCount int
	orientationCount     int
	accelerationCount    int
	magnetometerCount    int
	reconfCount          int
}

func (m *mock) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	m.angularVelocityCount++
	return av, nil
}

func (m *mock) ReadOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	m.orientationCount++
	return ea, nil
}

func (m *mock) ReadAcceleration(ctx context.Context) (r3.Vector, error) {
	m.accelerationCount++
	return ac, nil
}

func (m *mock) ReadMagnetometer(ctx context.Context) (r3.Vector, error) {
	m.magnetometerCount++
	return mg, nil
}

func (m *mock) GetReadings(ctx context.Context) ([]interface{}, error) {
	return imu.GetReadings(ctx, m)
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

type mockWithSensor struct {
	mock
	readingsCount int
}

func (m *mockWithSensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	m.readingsCount++
	return readings, nil
}
