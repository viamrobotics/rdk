package servo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mitchellh/mapstructure"
	"go.viam.com/test"
	"go.viam.com/utils"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/servo"
	pb "go.viam.com/rdk/proto/api/component/servo/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testServoName    = "servo1"
	testServoName2   = "servo2"
	failServoName    = "servo3"
	fakeServoName    = "servo4"
	missingServoName = "servo5"
)

func setupInjectRobot() *inject.Robot {
	servo1 := &mock{Name: testServoName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case servo.Named(testServoName):
			return servo1, nil
		case servo.Named(fakeServoName):
			return "not a servo", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{servo.Named(testServoName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	s, err := servo.FromRobot(r, testServoName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := s.Do(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	s, err := servo.FromRobot(r, testServoName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	result, err := s.GetPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, pos)

	s, err = servo.FromRobot(r, fakeServoName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Servo", "string"))
	test.That(t, s, test.ShouldBeNil)

	s, err = servo.FromRobot(r, missingServoName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(servo.Named(missingServoName)))
	test.That(t, s, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := servo.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testServoName})
}

func TestStatusValid(t *testing.T) {
	status := &pb.Status{PositionDeg: uint32(8), IsMoving: true}
	map1, err := protoutils.InterfaceToMap(status)
	test.That(t, err, test.ShouldBeNil)
	newStruct, err := structpb.NewStruct(map1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newStruct.AsMap(), test.ShouldResemble, map[string]interface{}{"position_deg": 8.0, "is_moving": true})

	convMap := &pb.Status{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}

func TestCreateStatus(t *testing.T) {
	_, err := servo.CreateStatus(context.Background(), "not a servo")
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalServo", "string"))

	status := &pb.Status{PositionDeg: uint32(8), IsMoving: true}

	injectServo := &inject.Servo{}
	injectServo.GetPositionFunc = func(ctx context.Context) (uint8, error) {
		return uint8(status.PositionDeg), nil
	}
	injectServo.IsMovingFunc = func() bool {
		return true
	}

	t.Run("working", func(t *testing.T) {
		status1, err := servo.CreateStatus(context.Background(), injectServo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)

		resourceSubtype := registry.ResourceSubtypeLookup(servo.Subtype)
		status2, err := resourceSubtype.Status(context.Background(), injectServo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status)
	})

	t.Run("not moving", func(t *testing.T) {
		injectServo.IsMovingFunc = func() bool {
			return false
		}

		status2 := &pb.Status{PositionDeg: uint32(8), IsMoving: false}
		status2, err := servo.CreateStatus(context.Background(), injectServo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status2)
	})

	t.Run("fail on GetPosition", func(t *testing.T) {
		errFail := errors.New("can't get position")
		injectServo.GetPositionFunc = func(ctx context.Context) (uint8, error) {
			return 0, errFail
		}
		_, err = servo.CreateStatus(context.Background(), injectServo)
		test.That(t, err, test.ShouldBeError, errFail)
	})
}

func TestServoName(t *testing.T) {
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
					ResourceSubtype: servo.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testServoName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: servo.SubtypeName,
				},
				Name: testServoName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := servo.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualServo1 servo.Servo = &mock{Name: testServoName}
	reconfServo1, err := servo.WrapWithReconfigurable(actualServo1)
	test.That(t, err, test.ShouldBeNil)

	_, err = servo.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalServo", nil))

	reconfServo2, err := servo.WrapWithReconfigurable(reconfServo1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfServo2, test.ShouldEqual, reconfServo1)
}

func TestReconfigurableServo(t *testing.T) {
	actualServo1 := &mock{Name: testServoName}
	reconfServo1, err := servo.WrapWithReconfigurable(actualServo1)
	test.That(t, err, test.ShouldBeNil)

	actualServo2 := &mock{Name: testServoName2}
	reconfServo2, err := servo.WrapWithReconfigurable(actualServo2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 0)

	err = reconfServo1.Reconfigure(context.Background(), reconfServo2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfServo1, test.ShouldResemble, reconfServo2)
	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualServo1.posCount, test.ShouldEqual, 0)
	test.That(t, actualServo2.posCount, test.ShouldEqual, 0)
	result, err := reconfServo1.(servo.Servo).GetPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, pos)
	test.That(t, actualServo1.posCount, test.ShouldEqual, 0)
	test.That(t, actualServo2.posCount, test.ShouldEqual, 1)

	err = reconfServo1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *servo.reconfigurableServo")
}

func TestStop(t *testing.T) {
	actualServo1 := &mock{Name: testServoName}
	reconfServo1, err := servo.WrapWithReconfigurable(actualServo1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualServo1.stopCount, test.ShouldEqual, 0)
	test.That(t, reconfServo1.(servo.Servo).Stop(context.Background()), test.ShouldBeNil)
	test.That(t, actualServo1.stopCount, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualServo1 := &mock{Name: testServoName}
	reconfServo1, err := servo.WrapWithReconfigurable(actualServo1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfServo1), test.ShouldBeNil)
	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 1)
}

const pos = 3

type mock struct {
	servo.LocalServo
	Name string

	posCount    int
	stopCount   int
	reconfCount int
}

func (mServo *mock) Move(ctx context.Context, angleDegs uint8) error {
	return nil
}

func (mServo *mock) GetPosition(ctx context.Context) (uint8, error) {
	mServo.posCount++
	return pos, nil
}

func (mServo *mock) Stop(ctx context.Context) error {
	mServo.stopCount++
	return nil
}

func (mServo *mock) Close() { mServo.reconfCount++ }

func (mServo *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
