package rtkutils

import (
	"context"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var (
	alt   = 50.5
	speed = 5.4
	fix   = 1
)

const (
	testRoverName   = "testRover"
	testStationName = "testStation"
	testBoardName   = "board1"
	testBusName     = "bus1"
	testi2cAddr     = 44
)

func setupInjectRobotWithGPS() *inject.Robot {
	r := &inject.Robot{}

	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		switch name {
		case movementsensor.Named(testRoverName):
			return &RTKMovementSensor{}, nil
		default:
			return nil, resource.NewNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{movementsensor.Named(testRoverName), movementsensor.Named(testStationName)}
	}
	return r
}

func TestModelTypeCreators(t *testing.T) {
	r := setupInjectRobotWithGPS()
	gps1, err := movementsensor.FromRobot(r, testRoverName)
	test.That(t, gps1, test.ShouldResemble, &RTKMovementSensor{})
	test.That(t, err, test.ShouldBeNil)
}

func TestReadingsRTK(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKMovementSensor{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	g.Nmeamovementsensor = &fake.MovementSensor{}

	status, err := g.NtripStatus()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldEqual, false)

	loc1, alt1, err := g.Position(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldResemble, geo.NewPoint(40.7, -73.98))
	test.That(t, alt1, test.ShouldEqual, alt)

	speed1, err := g.LinearVelocity(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1.Y, test.ShouldEqual, speed)

	fix1, err := g.ReadFix(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fix1, test.ShouldEqual, fix)
}

func TestCloseRTK(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKMovementSensor{
		cancelCtx:   cancelCtx,
		cancelFunc:  cancelFunc,
		logger:      logger,
		ntripClient: &NtripInfo{},
	}
	g.Nmeamovementsensor = &fake.MovementSensor{}

	err := g.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
