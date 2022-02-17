package motor_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testMotorName    = "motor1"
	testMotorName2   = "motor2"
	failMotorName    = "motor3"
	fakeMotorName    = "motor4"
	missingMotorName = "motor5"
)

func setupInjectRobot() *inject.Robot {
	motor1 := &mock{Name: testMotorName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		switch name {
		case motor.Named(testMotorName):
			return motor1, true
		case motor.Named(fakeMotorName):
			return "not a motor", true
		default:
			return nil, false
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{motor.Named(testMotorName), arm.Named("arm1")}
	}
	return r
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := motor.FromRobot(r, testMotorName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, err := res.GetPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, position)

	res, err = motor.FromRobot(r, fakeMotorName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Motor", "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = motor.FromRobot(r, missingMotorName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(motor.Named(missingMotorName)))
	test.That(t, res, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := motor.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testMotorName})
}

func TestMotorName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "a5ec0320-f103-5dd8-b56c-e9f363fb792a",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: motor.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testMotorName,
			resource.Name{
				UUID: "e0fbfb5f-147a-5e4d-b209-ca362547c8cf",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: motor.SubtypeName,
				},
				Name: testMotorName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := motor.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualMotor1 motor.Motor = &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	_, err = motor.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Motor", nil))

	reconfMotor2, err := motor.WrapWithReconfigurable(reconfMotor1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMotor2, test.ShouldEqual, reconfMotor1)
}

func TestReconfigurableMotor(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	actualMotor2 := &mock{Name: testMotorName2}
	reconfMotor2, err := motor.WrapWithReconfigurable(actualMotor2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.reconfCount, test.ShouldEqual, 0)

	err = reconfMotor1.Reconfigure(context.Background(), reconfMotor2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMotor1, test.ShouldResemble, reconfMotor2)
	test.That(t, actualMotor1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualMotor1.posCount, test.ShouldEqual, 0)
	test.That(t, actualMotor2.posCount, test.ShouldEqual, 0)
	result, err := reconfMotor1.(motor.Motor).GetPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, position)
	test.That(t, actualMotor1.posCount, test.ShouldEqual, 0)
	test.That(t, actualMotor2.posCount, test.ShouldEqual, 1)

	err = reconfMotor1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *motor.reconfigurableMotor")
}

func TestSetPower(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.powerCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).SetPower(context.Background(), 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.powerCount, test.ShouldEqual, 1)
}

func TestGoFor(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.goForCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).GoFor(context.Background(), 0, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.goForCount, test.ShouldEqual, 1)
}

func TestGoTo(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.goToCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).GoTo(context.Background(), 0, 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.goToCount, test.ShouldEqual, 1)
}

func TestResetZeroPosition(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.zeroCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).ResetZeroPosition(context.Background(), 0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.zeroCount, test.ShouldEqual, 1)
}

func TestGetPosition(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.posCount, test.ShouldEqual, 0)
	pos1, err := reconfMotor1.(motor.Motor).GetPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos1, test.ShouldResemble, position)
	test.That(t, actualMotor1.posCount, test.ShouldEqual, 1)
}

func TestGetFeatures(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.featuresCount, test.ShouldEqual, 0)
	feat1, err := reconfMotor1.(motor.Motor).GetFeatures(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, feat1, test.ShouldResemble, features)
	test.That(t, actualMotor1.featuresCount, test.ShouldEqual, 1)
}

func TestStop(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.stopCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).Stop(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.stopCount, test.ShouldEqual, 1)
}

func TestIsPowered(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.poweredCount, test.ShouldEqual, 0)
	isPowered1, err := reconfMotor1.(motor.Motor).IsPowered(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isPowered1, test.ShouldEqual, isPowered)
	test.That(t, actualMotor1.poweredCount, test.ShouldEqual, 1)
}

func TestGoTillStop(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	err = reconfMotor1.(motor.LocalMotor).GoTillStop(context.Background(), 0, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not support GoTillStop")

	actualMotor2 := &mockLocal{Name: testMotorName}
	reconfMotor2, err := motor.WrapWithReconfigurable(actualMotor2)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor2.goTillStopCount, test.ShouldEqual, 0)
	err = reconfMotor2.(motor.LocalMotor).GoTillStop(context.Background(), 0, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor2.goTillStopCount, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfMotor1), test.ShouldBeNil)
	test.That(t, actualMotor1.reconfCount, test.ShouldEqual, 1)
}

var (
	position  = 5.5
	features  = map[motor.Feature]bool{motor.PositionReporting: true}
	isPowered = true
)

type mock struct {
	Name string

	powerCount    int
	goForCount    int
	goToCount     int
	zeroCount     int
	posCount      int
	featuresCount int
	stopCount     int
	poweredCount  int
	reconfCount   int
}

func (m *mock) SetPower(ctx context.Context, powerPct float64) error {
	m.powerCount++
	return nil
}

func (m *mock) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	m.goForCount++
	return nil
}

func (m *mock) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	m.goToCount++
	return nil
}

func (m *mock) ResetZeroPosition(ctx context.Context, offset float64) error {
	m.zeroCount++
	return nil
}

func (m *mock) GetPosition(ctx context.Context) (float64, error) {
	m.posCount++
	return position, nil
}

func (m *mock) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	m.featuresCount++
	return features, nil
}

func (m *mock) Stop(ctx context.Context) error {
	m.stopCount++
	return nil
}

func (m *mock) IsPowered(ctx context.Context) (bool, error) {
	m.poweredCount++
	return isPowered, nil
}

func (m *mock) Close() { m.reconfCount++ }

type mockLocal struct {
	mock
	Name string

	goTillStopCount int
}

func (m *mockLocal) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	m.goTillStopCount++
	return nil
}
