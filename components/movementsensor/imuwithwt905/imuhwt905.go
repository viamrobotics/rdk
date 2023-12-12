// Package imuwithwt905 implements wit imu HWT905
package imuwithwt905

/*
Sensor Manufacturer:  		Wit-motion
Supported Sensor Models: 	HWT905
Supported OS: Linux
This driver only supports HWT905-TTL model of Wit imu.
Tested Sensor Models and User Manuals:
	HWT905 TTL: https://drive.google.com/file/d/1RV7j8yzZjPsPmvQY--1UHr_FhBzc2YwO/view
*/

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("imu-wit-hwt905")

var baudRateList = []uint{115200, 9600, 0}

// max tilt to use tilt compensation is 45 degrees.
var maxTiltInRad = rutils.DegToRad(45)

// Config is used for converting a witmotion IMU MovementSensor config attributes.
type Config struct {
	Port     string `json:"serial_path"`
	BaudRate uint   `json:"serial_baud_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	// Validating serial path
	if cfg.Port == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "serial_path")
	}

	// Validating baud rate
	if !rutils.ValidateBaudRate(baudRateList, int(cfg.BaudRate)) {
		return nil, resource.NewConfigValidationError(path, errors.Errorf("Baud rate is not in %v", baudRateList))
	}

	return nil, nil
}

func init() {
	resource.RegisterComponent(movementsensor.API, model, resource.Registration[movementsensor.MovementSensor, *Config]{
		Constructor: newWit,
	})
}

type wit struct {
	resource.Named
	angularVelocity         spatialmath.AngularVelocity
	orientation             spatialmath.EulerAngles
	acceleration            r3.Vector
	magnetometer            r3.Vector
	compassheading          float64
	numBadReadings          uint32
	err                     movementsensor.LastError
	hasMagnetometer         bool
	mu                      sync.Mutex
	reconfigMu              sync.Mutex
	port                    io.ReadWriteCloser
	cancelFunc              func()
	cancelCtx               context.Context
	activeBackgroundWorkers sync.WaitGroup
	logger                  logging.Logger
	baudRate                uint
}

func (imu *wit) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	imu.reconfigMu.Lock()
	defer imu.reconfigMu.Unlock()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	if !rutils.ValidateBaudRate(baudRateList, int(newConf.BaudRate)) {
		imu.baudRate = 9600
		imu.logger.Debug("Setting default baudRate 9600")
	}

	return nil
}
func (imu *wit) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return imu.angularVelocity, imu.err.Get()
}

func (imu *wit) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
}

func (imu *wit) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return &imu.orientation, imu.err.Get()
}

// LinearAcceleration returns linear acceleration in mm_per_sec_per_sec.
func (imu *wit) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return imu.acceleration, imu.err.Get()
}

// getMagnetometer returns magnetic field in gauss.
func (imu *wit) getMagnetometer() (r3.Vector, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return imu.magnetometer, imu.err.Get()
}

func (imu *wit) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	var err error

	imu.mu.Lock()
	defer imu.mu.Unlock()

	// this will compensate for a tilted IMU if the tilt is less than 45 degrees
	// do not let the imu near permanent magnets or things that make a strong magnetic field
	imu.compassheading = imu.calculateCompassHeading()

	return imu.compassheading, err
}

// Helper function to calculate compass heading with tilt compensation.
func (imu *wit) calculateCompassHeading() float64 {
	pitch := imu.orientation.Pitch
	roll := imu.orientation.Roll

	var x, y float64

	// Tilt compensation only works if the pitch and roll are between -45 and 45 degrees.
	if math.Abs(roll) <= maxTiltInRad && math.Abs(pitch) <= maxTiltInRad {
		x, y = imu.calculateTiltCompensation(roll, pitch)
	} else {
		x = imu.magnetometer.X
		y = imu.magnetometer.Y
	}

	// calculate -180 to 180 heading from radians
	// North (y) is 0 so  the π/2 - atan2(y, x) identity is used
	// directly
	rad := math.Atan2(y, x) // -180 to 180 heading
	compass := rutils.RadToDeg(rad)
	compass = math.Mod(compass, 360)
	compass = math.Mod(compass+360, 360) // compass 0 to 360

	return compass
}

func (imu *wit) calculateTiltCompensation(roll, pitch float64) (float64, float64) {
	// calculate adjusted magnetometer readings. These get less accurate as the tilt angle increases.
	xComp := imu.magnetometer.X*math.Cos(pitch) + imu.magnetometer.Z*math.Sin(pitch)
	yComp := imu.magnetometer.X*math.Sin(roll)*math.Sin(pitch) +
		imu.magnetometer.Y*math.Cos(roll) - imu.magnetometer.Z*math.Sin(roll)*math.Cos(pitch)

	return xComp, yComp
}

func (imu *wit) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return geo.NewPoint(0, 0), 0, movementsensor.ErrMethodUnimplementedPosition
}

func (imu *wit) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (imu *wit) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.DefaultAPIReadings(ctx, imu, extra)
	if err != nil {
		return nil, err
	}

	mag, err := imu.getMagnetometer()
	if err != nil {
		return nil, err
	}
	readings["magnetometer"] = mag

	return readings, err
}

func (imu *wit) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()

	return &movementsensor.Properties{
		AngularVelocitySupported:    true,
		OrientationSupported:        true,
		LinearAccelerationSupported: true,
		CompassHeadingSupported:     imu.hasMagnetometer,
	}, nil
}

// newWit creates a new Wit IMU.
func newWit(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	i := wit{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		err:        movementsensor.NewLastError(1, 1),
		cancelFunc: cancelFunc,
		cancelCtx:  cancelCtx,
	}

	options := slib.OpenOptions{
		PortName:        newConf.Port,
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	if err := i.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	if newConf.BaudRate > 0 {
		options.BaudRate = i.baudRate
	} else {
		logger.Warnf(
			"no valid serial_baud_rate set, setting to default of %d, baud rate of wit imus are: %v", options.BaudRate, baudRateList,
		)
	}

	logger.Debugf("initializing wit serial connection with parameters: %+v", options)
	i.port, err = slib.Open(options)
	if err != nil {
		return nil, err
	}

	portReader := bufio.NewReader(i.port)
	i.startUpdateLoop(context.Background(), portReader, logger)

	return &i, nil
}

func (imu *wit) startUpdateLoop(ctx context.Context, portReader *bufio.Reader, logger logging.Logger) {
	imu.hasMagnetometer = false
	imu.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer utils.UncheckedErrorFunc(func() error {
			if imu.port != nil {
				if err := imu.port.Close(); err != nil {
					imu.port = nil
					return err
				}
				imu.port = nil
			}
			return nil
		})
		defer imu.activeBackgroundWorkers.Done()

		for {

			if imu.cancelCtx.Err() != nil {
				return
			}

			select {
			case <-imu.cancelCtx.Done():
				return
			default:
			}

			line, err := portReader.ReadString('U')
			func() {
				switch {
				case err != nil:
					logger.Error(err)
				case len(line) != 11:
					imu.numBadReadings++
					return
				default:
					imu.err.Set(imu.parseWIT(line))
				}
			}()
		}
	})
}

func scale(a, b byte, r float64) float64 {
	x := float64(int(b)<<8|int(a)) / 32768.0 // 0 -> 2
	x *= r                                   // 0 -> 2r
	x += r
	x = math.Mod(x, r*2)
	x -= r
	return x
}

func convertMagByteToTesla(a, b byte) float64 {
	x := float64(int(int8(b))<<8 | int(a)) // 0 -> 2
	return x
}

func (imu *wit) parseWIT(line string) error {
	if line[0] == 0x52 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu angularVelocity %d %v", len(line), line)
		}
		imu.angularVelocity.X = scale(line[1], line[2], 2000)
		imu.angularVelocity.Y = scale(line[3], line[4], 2000)
		imu.angularVelocity.Z = scale(line[5], line[6], 2000)
	}

	if line[0] == 0x53 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu orientation %d %v", len(line), line)
		}

		imu.orientation.Roll = rutils.DegToRad(scale(line[1], line[2], 180))
		imu.orientation.Pitch = rutils.DegToRad(scale(line[3], line[4], 180))
		imu.orientation.Yaw = rutils.DegToRad(scale(line[5], line[6], 180))
	}

	if line[0] == 0x51 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu acceleration %d %v", len(line), line)
		}
		imu.acceleration.X = scale(line[1], line[2], 16) * 9.80665 // converts to m_per_sec_per_sec in NYC
		imu.acceleration.Y = scale(line[3], line[4], 16) * 9.80665
		imu.acceleration.Z = scale(line[5], line[6], 16) * 9.80665
	}

	if line[0] == 0x54 {
		imu.hasMagnetometer = true
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu magnetometer %d %v", len(line), line)
		}
		imu.magnetometer.X = convertMagByteToTesla(line[1], line[2]) // converts uT (micro Tesla)
		imu.magnetometer.Y = convertMagByteToTesla(line[3], line[4])
		imu.magnetometer.Z = convertMagByteToTesla(line[5], line[6])
	}

	return nil
}

// Close shuts down wit and closes imu.port.
func (imu *wit) Close(ctx context.Context) error {
	imu.logger.Debug("Closing wit motion imu")
	imu.cancelFunc()
	imu.activeBackgroundWorkers.Wait()
	imu.logger.Debug("Closed wit motion imu")
	return imu.err.Get()
}
