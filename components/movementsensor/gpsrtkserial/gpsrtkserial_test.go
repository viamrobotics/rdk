package gpsrtkserial

import (
	"context"
	"math"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor/fake"
	rtk "go.viam.com/rdk/components/movementsensor/rtkutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestValidateRTK(t *testing.T) {
	path := "path"
	t.Run("valid config", func(t *testing.T) {
		cfg := Config{
			NtripURL:             "http//fakeurl",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			SerialPath:           path,
			SerialBaudRate:       115200,
		}
		err := cfg.validateNtrip(path)
		test.That(t, err, test.ShouldBeNil)
		err = cfg.validateSerialPath(path)
		test.That(t, err, test.ShouldBeNil)
		_, err = cfg.Validate(path)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := Config{
			NtripURL:             "",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			SerialPath:           path,
			SerialBaudRate:       115200,
		}

		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "ntrip_url"))
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := Config{
			NtripURL:             "http//fakeurl",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			SerialPath:           "",
			SerialBaudRate:       115200,
		}

		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "serial_path"))
	})
}

func TestConnect(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := rtkSerial{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	url := "http://fakeurl"
	username := "user"
	password := "pwd"

	err := g.connect("invalidurl", username, password, 10)
	test.That(t, err.Error(), test.ShouldContainSubstring, `address must start with http://`)

	g.ntripClient = makeMockNtripClient()

	err = g.connect(url, username, password, 10)
	test.That(t, err, test.ShouldBeNil)
}

func TestReadings(t *testing.T) {
	var (
		alt   = 50.5
		speed = 5.4
		loc   = geo.NewPoint(40.7, -73.98)
	)

	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := rtkSerial{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	mockSensor := &CustomMovementSensor{
		MovementSensor: &fake.MovementSensor{},
	}

	mockSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return loc, alt, nil
	}

	g.nmeamovementsensor = mockSensor

	// Normal position
	loc1, alt1, err := g.Position(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldResemble, loc)
	test.That(t, alt1, test.ShouldEqual, alt)

	speed1, err := g.LinearVelocity(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1.Y, test.ShouldEqual, speed)
	test.That(t, speed1.X, test.ShouldEqual, 0)
	test.That(t, speed1.Z, test.ShouldEqual, 0)

	// Zero position with latitude 0 and longitude 0.
	mockSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return geo.NewPoint(0, 0), 0, nil
	}

	loc2, alt2, err := g.Position(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc2, test.ShouldResemble, geo.NewPoint(0, 0))
	test.That(t, alt2, test.ShouldEqual, 0)

	speed2, err := g.LinearVelocity(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed2.Y, test.ShouldEqual, speed)
	test.That(t, speed2.X, test.ShouldEqual, 0)
	test.That(t, speed2.Z, test.ShouldEqual, 0)

	// Position with NaN values.
	mockSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return geo.NewPoint(math.NaN(), math.NaN()), math.NaN(), nil
	}
	g.lastposition.SetLastPosition(loc1)

	loc3, alt3, err := g.Position(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)

	// last known valid position should be returned when current position is NaN()
	test.That(t, loc3, test.ShouldResemble, loc1)
	test.That(t, math.IsNaN(alt3), test.ShouldBeTrue)

	speed3, err := g.LinearVelocity(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed3.Y, test.ShouldEqual, speed)
}

func TestReconfigure(t *testing.T) {
	g := &rtkSerial{
		writePath: "/dev/ttyUSB0",
		wbaud:     9600,
		logger:    logging.NewTestLogger(t),
	}

	conf := resource.Config{
		Name: "reconfig1",
		ConvertedAttributes: &Config{
			SerialPath:           "/dev/ttyUSB1",
			SerialBaudRate:       115200,
			NtripURL:             "http//fakeurl",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
		},
	}

	err := g.Reconfigure(context.Background(), nil, conf)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.writePath, test.ShouldResemble, "/dev/ttyUSB1")
	test.That(t, g.wbaud, test.ShouldEqual, 115200)
}

func TestCloseRTK(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := rtkSerial{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}
	g.ntripClient = &rtk.NtripInfo{}
	g.nmeamovementsensor = &fake.MovementSensor{}

	err := g.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

type CustomMovementSensor struct {
	*fake.MovementSensor
	PositionFunc func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error)
}

func (c *CustomMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	if c.PositionFunc != nil {
		return c.PositionFunc(ctx, extra)
	}
	// Fallback to the default implementation if PositionFunc is not set.
	return c.MovementSensor.Position(ctx, extra)
}

// mock ntripinfo client.
func makeMockNtripClient() *rtk.NtripInfo {
	return &rtk.NtripInfo{}
}
