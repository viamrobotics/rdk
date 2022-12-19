package motor_test

import (
	"context"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/motor/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/registry"
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

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	deps[motor.Named(testMotorName)] = &mock{Name: testMotorName}
	deps[motor.Named(fakeMotorName)] = "not a motor"
	return deps
}

func setupInjectRobot() *inject.Robot {
	motor1 := &mock{Name: testMotorName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case motor.Named(testMotorName):
			return motor1, nil
		case motor.Named(fakeMotorName):
			return "not a motor", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{motor.Named(testMotorName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	m, err := motor.FromRobot(r, testMotorName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := m.DoCommand(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	res, err := motor.FromDependencies(deps, testMotorName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, err := res.Position(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, position)

	res, err = motor.FromDependencies(deps, fakeMotorName)
	test.That(t, err, test.ShouldBeError, motor.DependencyTypeError(fakeMotorName, "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = motor.FromDependencies(deps, missingMotorName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyNotFoundError(missingMotorName))
	test.That(t, res, test.ShouldBeNil)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := motor.FromRobot(r, testMotorName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, err := res.Position(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, position)

	res, err = motor.FromRobot(r, fakeMotorName)
	test.That(t, err, test.ShouldBeError, motor.NewUnimplementedInterfaceError("string"))
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

func TestStatusValid(t *testing.T) {
	status := &pb.Status{IsPowered: true, Position: 7.7, IsMoving: true}
	newStruct, err := protoutils.StructToStructPb(status)
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		newStruct.AsMap(),
		test.ShouldResemble,
		map[string]interface{}{"is_powered": true, "position": 7.7, "is_moving": true},
	)

	convMap := &pb.Status{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)

	status = &pb.Status{Position: 7.7}
	newStruct, err = protoutils.StructToStructPb(status)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newStruct.AsMap(), test.ShouldResemble, map[string]interface{}{"position": 7.7})

	convMap = &pb.Status{}
	decoder, err = mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}

func TestCreateStatus(t *testing.T) {
	_, err := motor.CreateStatus(context.Background(), "not a motor")
	test.That(t, err, test.ShouldBeError, motor.NewUnimplementedLocalInterfaceError("string"))

	status := &pb.Status{IsPowered: true, Position: 7.7, IsMoving: true}

	injectMotor := &inject.LocalMotor{}
	injectMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
		return status.IsPowered, 1.0, nil
	}
	injectMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
		return map[motor.Feature]bool{motor.PositionReporting: true}, nil
	}
	injectMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return status.Position, nil
	}
	injectMotor.IsMovingFunc = func(context.Context) (bool, error) {
		return true, nil
	}

	t.Run("working", func(t *testing.T) {
		status1, err := motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)

		resourceSubtype := registry.ResourceSubtypeLookup(motor.Subtype)
		status2, err := resourceSubtype.Status(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status)
	})

	t.Run("not moving", func(t *testing.T) {
		injectMotor.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}

		status2 := &pb.Status{IsPowered: true, Position: 7.7, IsMoving: false}
		status1, err := motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status2)
	})

	t.Run("fail on Position", func(t *testing.T) {
		errFail := errors.New("can't get position")
		injectMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return 0, errFail
		}
		_, err = motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeError, errFail)
	})

	t.Run("position not supported", func(t *testing.T) {
		injectMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return map[motor.Feature]bool{motor.PositionReporting: false}, nil
		}

		status1, err := motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, &pb.Status{IsPowered: true})
	})

	t.Run("fail on Properties", func(t *testing.T) {
		errFail := errors.New("can't get features")
		injectMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
			return nil, errFail
		}
		_, err = motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeError, errFail)
	})

	t.Run("fail on IsPowered", func(t *testing.T) {
		errFail := errors.New("can't get is powered")
		injectMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
			return false, 0.0, errFail
		}
		_, err = motor.CreateStatus(context.Background(), injectMotor)
		test.That(t, err, test.ShouldBeError, errFail)
	})
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
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = motor.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, motor.NewUnimplementedInterfaceError(nil))

	reconfMotor2, err := motor.WrapWithReconfigurable(reconfMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMotor2, test.ShouldEqual, reconfMotor1)

	var actualMotor2 motor.LocalMotor = &mockLocal{Name: testMotorName}
	reconfMotor3, err := motor.WrapWithReconfigurable(actualMotor2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	reconfMotor4, err := motor.WrapWithReconfigurable(reconfMotor3, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMotor4, test.ShouldResemble, reconfMotor3)

	_, ok := reconfMotor4.(motor.LocalMotor)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestReconfigurableMotor(t *testing.T) {
	actualMotor1 := &mockLocal{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	actualMotor2 := &mockLocal{Name: testMotorName2}
	reconfMotor2, err := motor.WrapWithReconfigurable(actualMotor2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.reconfCount, test.ShouldEqual, 0)

	err = reconfMotor1.Reconfigure(context.Background(), reconfMotor2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMotor1, test.ShouldResemble, reconfMotor2)
	test.That(t, actualMotor1.reconfCount, test.ShouldEqual, 2)

	test.That(t, actualMotor1.posCount, test.ShouldEqual, 0)
	test.That(t, actualMotor2.posCount, test.ShouldEqual, 0)
	result, err := reconfMotor1.(motor.Motor).Position(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, position)
	test.That(t, actualMotor1.posCount, test.ShouldEqual, 0)
	test.That(t, actualMotor2.posCount, test.ShouldEqual, 1)

	err = reconfMotor1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfMotor1, nil))

	actualMotor3 := &mock{Name: failMotorName}
	reconfMotor3, err := motor.WrapWithReconfigurable(actualMotor3, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMotor3, test.ShouldNotBeNil)

	err = reconfMotor1.Reconfigure(context.Background(), reconfMotor3)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfMotor1, reconfMotor3))
	test.That(t, actualMotor3.reconfCount, test.ShouldEqual, 0)

	err = reconfMotor3.Reconfigure(context.Background(), reconfMotor1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfMotor3, reconfMotor1))

	actualMotor4 := &mock{Name: testMotorName2}
	reconfMotor4, err := motor.WrapWithReconfigurable(actualMotor4, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMotor4, test.ShouldNotBeNil)

	err = reconfMotor3.Reconfigure(context.Background(), reconfMotor4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfMotor3, test.ShouldResemble, reconfMotor4)
}

func TestSetPower(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.powerCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).SetPower(context.Background(), 0, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.powerCount, test.ShouldEqual, 1)
}

func TestGoFor(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.goForCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).GoFor(context.Background(), 0, 0, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.goForCount, test.ShouldEqual, 1)
}

func TestGoTo(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.goToCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).GoTo(context.Background(), 0, 0, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.goToCount, test.ShouldEqual, 1)
}

func TestResetZeroPosition(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.zeroCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).ResetZeroPosition(context.Background(), 0, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.zeroCount, test.ShouldEqual, 1)
}

func TestPosition(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.posCount, test.ShouldEqual, 0)
	pos1, err := reconfMotor1.(motor.Motor).Position(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos1, test.ShouldResemble, position)
	test.That(t, actualMotor1.posCount, test.ShouldEqual, 1)
}

func TestProperties(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.featuresCount, test.ShouldEqual, 0)
	feat1, err := reconfMotor1.(motor.Motor).Properties(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, feat1, test.ShouldResemble, features)
	test.That(t, actualMotor1.featuresCount, test.ShouldEqual, 1)
}

func TestStop(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.stopCount, test.ShouldEqual, 0)
	err = reconfMotor1.(motor.Motor).Stop(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.stopCount, test.ShouldEqual, 1)
}

func TestIsPowered(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.poweredCount, test.ShouldEqual, 0)
	isPowered1, _, err := reconfMotor1.(motor.Motor).IsPowered(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isPowered1, test.ShouldEqual, isPowered)
	test.That(t, actualMotor1.poweredCount, test.ShouldEqual, 1)
}

func TestGoTillStop(t *testing.T) {
	actualMotor := &mockLocal{Name: testMotorName}
	reconfMotor, err := motor.WrapWithReconfigurable(actualMotor, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor.goTillStopCount, test.ShouldEqual, 0)
	err = reconfMotor.(motor.LocalMotor).GoTillStop(context.Background(), 0, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor.goTillStopCount, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfMotor1), test.ShouldBeNil)
	test.That(t, actualMotor1.reconfCount, test.ShouldEqual, 1)
}

func TestExtra(t *testing.T) {
	actualMotor1 := &mock{Name: testMotorName}
	reconfMotor1, err := motor.WrapWithReconfigurable(actualMotor1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMotor1.extra, test.ShouldEqual, nil)
	err = reconfMotor1.(motor.Motor).SetPower(context.Background(), 0, map[string]interface{}{"foo": "bar", "baz": [3]int{1, 2, 3}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualMotor1.extra, test.ShouldResemble, map[string]interface{}{"foo": "bar", "baz": [3]int{1, 2, 3}})
}

var (
	position     = 5.5
	features     = map[motor.Feature]bool{motor.PositionReporting: true}
	isPowered    = true
	mockPowerPct = 1.0
	isMoving     = true
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
	isMovingCount int
	reconfCount   int
	extra         map[string]interface{}
}

func (m *mock) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.powerCount++
	m.extra = extra
	mockPowerPct = powerPct
	return nil
}

func (m *mock) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	m.goForCount++
	m.extra = extra
	return nil
}

func (m *mock) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	m.goToCount++
	m.extra = extra
	return nil
}

func (m *mock) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	m.zeroCount++
	return nil
}

func (m *mock) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	m.posCount++
	m.extra = extra
	return position, nil
}

func (m *mock) Properties(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	m.featuresCount++
	m.extra = extra
	return features, nil
}

func (m *mock) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.stopCount++
	m.extra = extra
	return nil
}

func (m *mock) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	m.poweredCount++
	m.extra = extra
	return isPowered, mockPowerPct, nil
}

func (m *mock) IsMoving(ctx context.Context) (bool, error) {
	m.isMovingCount++
	return isMoving, nil
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

type mockLocal struct {
	mock
	Name string

	goTillStopCount int
}

func (m *mockLocal) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	m.goTillStopCount++
	return nil
}
