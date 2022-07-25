package posetracker_test

import (
	"context"
	"math"
	"sort"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/component/posetracker"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

const missingPTName = "dne"

func setupInjectRobot() *inject.Robot {
	poseTracker := &inject.PoseTracker{}
	poseTracker.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return cmd, nil
	}
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

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	p, err := posetracker.FromRobot(r, workingPTName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, p, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := p.Do(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
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
	return posetracker.BodyToPoseInFrame{
		"body1": referenceframe.NewPoseInFrame("world", spatialmath.NewZeroPose()),
		"body2": referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromOrientation(
			r3.Vector{X: 2, Y: 4, Z: 6},
			&spatialmath.R4AA{Theta: math.Pi, RX: 0, RY: 0, RZ: 1},
		)),
	}, nil
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

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

	expectedPoses := posetracker.BodyToPoseInFrame{
		"body1": referenceframe.NewPoseInFrame("world", spatialmath.NewZeroPose()),
		"body2": referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromOrientation(
			r3.Vector{X: 2, Y: 4, Z: 6},
			&spatialmath.R4AA{Theta: math.Pi, RX: 0, RY: 0, RZ: 1},
		)),
	}
	poses, err := reconfPT1.(posetracker.PoseTracker).GetPoses(context.Background(), []string{})
	test.That(t, poses, test.ShouldResemble, expectedPoses)
	test.That(t, err, test.ShouldBeNil)

	err = reconfPT1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestGetReadings(t *testing.T) {
	actualPT1 := &mock{Name: workingPTName}
	reconfPT1, err := posetracker.WrapWithReconfigurable(actualPT1)
	test.That(t, err, test.ShouldBeNil)
	sensorPT1, isSensor := reconfPT1.(sensor.Sensor)
	test.That(t, isSensor, test.ShouldBeTrue)

	expectedReadings := [][]interface{}{
		{"body1", "world", 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0},
		{"body2", "world", 2.0, 4.0, 6.0, 0.0, 0.0, 1.0, math.Pi},
	}
	receivedRawReadings, err := sensorPT1.GetReadings(context.Background())
	test.That(t, err, test.ShouldBeNil)

	receivedReadings := make([][]interface{}, 0)
	for _, reading := range receivedRawReadings {
		receivedReadings = append(receivedReadings, reading.([]interface{}))
	}
	sort.SliceStable(expectedReadings, func(i, j int) bool {
		return expectedReadings[i][0].(string) < expectedReadings[j][0].(string)
	})
	sort.SliceStable(receivedReadings, func(i, j int) bool {
		return receivedReadings[i][0].(string) < receivedReadings[j][0].(string)
	})

	test.That(t, receivedReadings, test.ShouldResemble, expectedReadings)
}

func TestClose(t *testing.T) {
	actualPT := &mock{Name: workingPTName}
	reconfPT, err := posetracker.WrapWithReconfigurable(actualPT)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualPT.reconfCount, test.ShouldEqual, 0)
	test.That(t, viamutils.TryClose(context.Background(), reconfPT), test.ShouldBeNil)
	test.That(t, actualPT.reconfCount, test.ShouldEqual, 1)
}
