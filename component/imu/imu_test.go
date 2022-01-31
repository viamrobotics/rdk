package imu_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testIMUName    = "imu1"
	testIMUName2   = "imu2"
	fakeIMUName    = "imu3"
	missingIMUName = "imu4"
)

func setupInjectRobot() *inject.Robot {
	imu1 := &mock{Name: testIMUName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		switch name {
		case imu.Named(testIMUName):
			return imu1, true
		case imu.Named(fakeIMUName):
			return "not a imu", false
		default:
			return nil, false
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{imu.Named(testIMUName), arm.Named("arm1")}
	}
	return r
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	s, ok := imu.FromRobot(r, testIMUName)
	test.That(t, s, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	result, err := s.ReadOrientation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, ea)

	s, ok = imu.FromRobot(r, fakeIMUName)
	test.That(t, s, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	s, ok = imu.FromRobot(r, missingIMUName)
	test.That(t, s, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
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
				UUID: "053e1e0c-20de-59e7-bace-922cb1ada629",
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
				UUID: "aed67198-6075-5806-837a-6d33ee4b5a42",
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
)

func TestWrapWithReconfigurable(t *testing.T) {
	var actualIMU1 imu.IMU = &mock{Name: testIMUName}
	reconfIMU1, err := imu.WrapWithReconfigurable(actualIMU1)
	test.That(t, err, test.ShouldBeNil)

	_, err = imu.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

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
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected new IMU")
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

func TestOrientiation(t *testing.T) {
	actualIMU1 := &mock{Name: testIMUName}
	reconfIMU1, _ := imu.WrapWithReconfigurable(actualIMU1)

	test.That(t, actualIMU1.orientationCount, test.ShouldEqual, 0)
	angles, err := reconfIMU1.(imu.IMU).ReadOrientation(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angles, test.ShouldResemble, &spatialmath.EulerAngles{Roll: 4, Pitch: 5, Yaw: 6})
	test.That(t, actualIMU1.orientationCount, test.ShouldEqual, 1)
}

func TestGetReadings(t *testing.T) {
	actualIMU1 := &mock{Name: testIMUName}
	reconfIMU1, _ := imu.WrapWithReconfigurable(actualIMU1)

	test.That(t, actualIMU1.readingsCount, test.ShouldEqual, 0)
	result, err := reconfIMU1.(imu.IMU).GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, []interface{}{av, ea})
	test.That(t, actualIMU1.readingsCount, test.ShouldEqual, 1)
}

type mock struct {
	imu.IMU
	Name                 string
	angularVelocityCount int
	orientationCount     int
	readingsCount        int
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

func (m *mock) GetReadings(ctx context.Context) ([]interface{}, error) {
	m.readingsCount++
	return []interface{}{av, ea}, nil
}

func (m *mock) Close() { m.reconfCount++ }
