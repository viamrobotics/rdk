package posetracker_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/component/posetracker"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

const missingPTName = "dne"

func setupInjectRobot() *inject.Robot {
	poseTracker := &inject.PoseTracker{}
	robot := &inject.Robot{}
	robot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case posetracker.Named(workingPTName):
			return poseTracker, nil
		case posetracker.Named(notPTName):
			return "not an arm", nil
		default:
			return nil, utils.NewResourceNotFoundError(name)
		}
	}
	robot.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{posetracker.Named(workingPTName), sensor.Named("sensor1")}
	}
	return robot
}

func TestFromRobot(t *testing.T) {
	robot := setupInjectRobot()

	t.Run("FromRobot works for properly implemented and registered pose tracker",
		func(t *testing.T) {
			poseTracker, err := posetracker.FromRobot(robot, workingPTName)
			_, isPoseTracker := poseTracker.(*inject.PoseTracker)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, isPoseTracker, test.ShouldBeTrue)
		},
	)

	t.Run("FromRobot fails on improperly implemented pose tracker", func(t *testing.T) {
		poseTracker, err := posetracker.FromRobot(robot, notPTName)
		test.That(
			t, err, test.ShouldBeError,
			utils.NewUnimplementedInterfaceError("PoseTracker", "string"),
		)
		test.That(t, poseTracker, test.ShouldBeNil)
	})

	t.Run("FromRobot fails on unregistered pose tracker", func(t *testing.T) {
		poseTracker, err := posetracker.FromRobot(robot, missingPTName)
		test.That(
			t, err, test.ShouldBeError,
			utils.NewResourceNotFoundError(posetracker.Named(missingPTName)),
		)
		test.That(t, poseTracker, test.ShouldBeNil)
	})
}

func TestPoseTrackerName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "064dbb8a-59a8-5d14-9be9-22eee1fab080",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: posetracker.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			workingPTName,
			resource.Name{
				UUID: "d5c66251-4392-541b-a99d-17f535048d36",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: posetracker.SubtypeName,
				},
				Name: workingPTName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := posetracker.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	poseTracker := &inject.PoseTracker{}
	reconfPT, err := posetracker.WrapWithReconfigurable(poseTracker)
	test.That(t, err, test.ShouldBeNil)
	_, err = posetracker.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, utils.NewUnimplementedInterfaceError("PoseTracker", nil))
	reconfPT2, err := posetracker.WrapWithReconfigurable(reconfPT)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfPT2, test.ShouldEqual, reconfPT)
}

type mock struct {
	posetracker.PoseTracker
	Name        string
	reconfCount int
}

func (m *mock) GetPoses(ctx context.Context, bodyNames []string) (posetracker.BodyToPoseInFrame, error) {
	return posetracker.BodyToPoseInFrame{}, nil
}

func (m *mock) Close() { m.reconfCount++ }

func TestReconfigurablePoseTracker(t *testing.T) {
	actualPT1 := &mock{Name: workingPTName}
	reconfPT1, err := posetracker.WrapWithReconfigurable(actualPT1)
	test.That(t, err, test.ShouldBeNil)

	actualPT2 := &mock{Name: "differentPT"}
	reconfPT2, err := posetracker.WrapWithReconfigurable(actualPT2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualPT1.reconfCount, test.ShouldEqual, 0)

	err = reconfPT1.Reconfigure(context.Background(), reconfPT2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfPT1, test.ShouldResemble, reconfPT2)
	test.That(t, actualPT1.reconfCount, test.ShouldEqual, 1)

	expectedPoses := posetracker.BodyToPoseInFrame{}
	poses, err := reconfPT1.(posetracker.PoseTracker).GetPoses(context.Background(), []string{})
	test.That(t, poses, test.ShouldResemble, expectedPoses)
	test.That(t, err, test.ShouldBeNil)

	err = reconfPT1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestClose(t *testing.T) {
	actualPT := &mock{Name: workingPTName}
	reconfPT, err := posetracker.WrapWithReconfigurable(actualPT)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualPT.reconfCount, test.ShouldEqual, 0)
	test.That(t, viamutils.TryClose(context.Background(), reconfPT), test.ShouldBeNil)
	test.That(t, actualPT.reconfCount, test.ShouldEqual, 1)
}
