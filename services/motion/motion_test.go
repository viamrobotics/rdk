package motion_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/gripper"
	// register.
	_ "go.viam.com/rdk/component/register"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testMotionServiceName  = "motion1"
	testMotionServiceName2 = "motion2"
)

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	return r, svc1
}

type mock struct {
	motion.Service
	grabCount   int
	name        string
	reconfCount int
}

func (m *mock) Move(
	ctx context.Context,
	gripperName resource.Name,
	grabPose *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	m.grabCount++
	return false, nil
}

func (m *mock) MoveSingleComponent(
	ctx context.Context,
	gripperName resource.Name,
	grabPose *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	m.grabCount++
	return false, nil
}

func (m *mock) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	return &referenceframe.PoseInFrame{}, nil
}

func (m *mock) Close(ctx context.Context) error {
	m.reconfCount++
	return nil
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := motion.FromRobot(r, testMotionServiceName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	grabPose := referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())
	result, err := svc.Move(context.Background(), gripper.Named("fake"), grabPose, &commonpb.WorldState{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, false)
	test.That(t, svc1.grabCount, test.ShouldEqual, 1)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return "not motion", nil
	}

	svc, err = motion.FromRobot(r, testMotionServiceName2)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("motion.Service", "string"))
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return nil, rutils.NewResourceNotFoundError(name)
	}

	svc, err = motion.FromRobot(r, testMotionServiceName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(motion.Named(testMotionServiceName)))
	test.That(t, svc, test.ShouldBeNil)
}

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(motion.Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	svc := &mock{name: "svc1"}
	reconfSvc1, err := motion.WrapWithReconfigurable(svc)
	test.That(t, err, test.ShouldBeNil)

	_, err = motion.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("motion.Service", nil))

	reconfSvc2, err := motion.WrapWithReconfigurable(reconfSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldEqual, reconfSvc1)
}

func TestReconfigurable(t *testing.T) {
	actualSvc1 := &mock{name: "svc1"}
	reconfSvc1, err := motion.WrapWithReconfigurable(actualSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldNotBeNil)

	actualArm2 := &mock{name: "svc2"}
	reconfSvc2, err := motion.WrapWithReconfigurable(actualArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldNotBeNil)
	test.That(t, actualSvc1.reconfCount, test.ShouldEqual, 0)

	err = reconfSvc1.Reconfigure(context.Background(), reconfSvc2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldResemble, reconfSvc2)
	test.That(t, actualSvc1.reconfCount, test.ShouldEqual, 1)

	err = reconfSvc1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfSvc1, nil))
}
