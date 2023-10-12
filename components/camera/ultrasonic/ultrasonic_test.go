package ultrasonic

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/sensor/ultrasonic"
	pointcloud "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testSensorName = "ultrasonic1"
	triggerPin     = "some-pin"
	echoInterrupt  = "some-echo-interrupt"
	board1         = "some-board"
)

func setupDependencies(t *testing.T) resource.Dependencies {
	t.Helper()

	deps := make(resource.Dependencies)

	actualBoard := inject.NewBoard(board1)
	actualBoard.DigitalInterruptNamesFunc = func() []string {
		return []string{echoInterrupt}
	}
	injectDigi := &inject.DigitalInterrupt{}
	actualBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) {
		return injectDigi, true
	}
	pin := &inject.GPIOPin{}
	pin.SetFunc = func(ctx context.Context, high bool, extra map[string]interface{}) error {
		return nil
	}
	actualBoard.GPIOPinByNameFunc = func(name string) (board.GPIOPin, error) {
		return pin, nil
	}
	deps[board.Named(board1)] = actualBoard

	return deps
}

func TestNewCamera(t *testing.T) {
	fakecfg := &ultrasonic.Config{TriggerPin: triggerPin, EchoInterrupt: echoInterrupt, Board: board1}
	name := resource.Name{API: camera.API}
	ctx := context.Background()
	deps := setupDependencies(t)
	logger := golog.NewTestLogger(t)
	_, err := newCamera(ctx, deps, name, fakecfg, logger)
	test.That(t, err, test.ShouldBeNil)
}

func TestUnderlyingSensor(t *testing.T) {
	name := resource.Name{API: camera.API}
	ctx := context.Background()

	fakeUS := inject.NewSensor("mySensor")
	fakeUS.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"distance": 3.2}, nil
	}
	logger := golog.NewTestLogger(t)
	cam, err := cameraFromSensor(ctx, name, fakeUS, logger)
	test.That(t, err, test.ShouldBeNil)

	pc, err := cam.NextPointCloud(ctx)
	test.That(t, err, test.ShouldBeNil)

	values := []float64{}
	count := 0

	pc.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		values = append(values, p.Z)
		count++
		return true
	})

	test.That(t, count, test.ShouldEqual, 1)
	test.That(t, values[0], test.ShouldEqual, 3.2)
	stream, err := cam.Stream(ctx)
	test.That(t, err, test.ShouldBeNil)

	_, _, err = stream.Next(ctx)
	test.That(t, err.Error(), test.ShouldEqual, "not yet implemented")
}
