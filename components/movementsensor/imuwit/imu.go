// Package imuwit implements wit imus.
package imuwit

/*
Sensor Manufacturer:  		Wit-motion
Supported Sensor Models: 	HWT901B, BWT901, BWT61CL
Supported OS: Linux
Tested Sensor Models and User Manuals:

	BWT61CL: https://drive.google.com/file/d/1cUTginKXArkHvwPB4LdqojG-ixm7PXCQ/view
	BWT901:	https://drive.google.com/file/d/18bScCGO5vVZYcEeNKjXNtjnT8OVlrHGI/view
	HWT901B TTL: https://drive.google.com/file/d/10HW4MhvhJs4RP0ko7w2nnzwmzsFCKPs6/view

This driver will connect to the sensor using a usb connection given as a serial path
using a default baud rate of 115200. We allow baud rate values of: 9600, 115200
The default baud rate can be used to connect to all models. Only the HWT901B's baud rate is changeable.
We ask the user to refer to the datasheet if any baud rate changes are required for their application.

Other models that connect over serial may work, but we ask the user to refer to wit-motion's datasheet
in that case as well. As of Feb 2023, Wit-motion has 48 gyro/inclinometer/imu models with varied levels of
driver commonality.
*/

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("imu-wit")

var baudRateList = [...]uint{115200, 9600, 0}

// AttrConfig is used for converting a witmotion IMU MovementSensor config attributes.
type AttrConfig struct {
	Port     string `json:"serial_path"`
	BaudRate uint   `json:"serial_baud_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) error {
	// Validating serial path
	if cfg.Port == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}

	// Validating baud rate
	isValid := false
	for _, val := range baudRateList {
		if val == cfg.BaudRate {
			isValid = true
		}
	}
	if !isValid {
		return utils.NewConfigValidationError(path, errors.Errorf("Baud rate is not in %v", baudRateList))
	}

	return nil
}

func init() {
	registry.RegisterComponent(movementsensor.Subtype, model, registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			cfg config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewWit(ctx, deps, cfg, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(movementsensor.Subtype, model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr AttrConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		},
		&AttrConfig{})
}

type wit struct {
	angularVelocity spatialmath.AngularVelocity
	orientation     spatialmath.EulerAngles
	acceleration    r3.Vector
	magnetometer    r3.Vector
	numBadReadings  uint32
	err             movementsensor.LastError

	mu sync.Mutex

	port                    io.ReadWriteCloser
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	generic.Unimplemented
	logger golog.Logger
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

// GetMagnetometer returns magnetic field in gauss.
func (imu *wit) GetMagnetometer(ctx context.Context) (r3.Vector, error) {
	imu.mu.Lock()
	defer imu.mu.Unlock()
	return imu.magnetometer, imu.err.Get()
}

func (imu *wit) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, movementsensor.ErrMethodUnimplementedCompassHeading
}

func (imu *wit) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return geo.NewPoint(0, 0), 0, movementsensor.ErrMethodUnimplementedPosition
}

func (imu *wit) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (imu *wit) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.Readings(ctx, imu, extra)
	if err != nil {
		return nil, err
	}

	mag, err := imu.GetMagnetometer(ctx)
	if err != nil {
		return nil, err
	}
	readings["magnetometer"] = mag

	return readings, err
}

func (imu *wit) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		AngularVelocitySupported:    true,
		OrientationSupported:        true,
		LinearAccelerationSupported: true,
	}, nil
}

// NewWit creates a new Wit IMU.
func NewWit(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	conf, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rutils.NewUnexpectedTypeError(conf, cfg.ConvertedAttributes)
	}

	options := slib.OpenOptions{
		PortName:        conf.Port,
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	if conf.BaudRate > 0 {
		options.BaudRate = conf.BaudRate
	} else {
		logger.Warnf(
			"no valid serial_baud_rate set, setting to default of %d, baud rate of wit imus are: %v", options.BaudRate, baudRateList,
		)
	}

	i := wit{logger: logger}
	logger.Debugf("initializing wit serial connection with parameters: %+v", options)
	var err error
	i.port, err = slib.Open(options)
	if err != nil {
		return nil, err
	}

	portReader := bufio.NewReader(i.port)
	i.startUpdateLoop(ctx, portReader, logger)

	return &i, nil
}

func (imu *wit) startUpdateLoop(ctx context.Context, portReader *bufio.Reader, logger golog.Logger) {
	ctx, imu.cancelFunc = context.WithCancel(ctx)
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
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := portReader.ReadString('U')

			func() {
				imu.mu.Lock()
				defer imu.mu.Unlock()

				switch {
				case err != nil:
					imu.err.Set(err)
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

func scalemag(a, b byte, r float64) float64 {
	x := float64(int(b)<<8 | int(a)) // 0 -> 2
	x *= r                           // 0 -> 2r
	x += r
	x = math.Mod(x, r*2)
	x -= r
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
		imu.acceleration.X = scale(line[1], line[2], 16) * 9806.65 // converts of mm_per_sec_per_sec in NYC
		imu.acceleration.Y = scale(line[3], line[4], 16) * 9806.65
		imu.acceleration.Z = scale(line[5], line[6], 16) * 9806.65
	}

	if line[0] == 0x54 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu magnetometer %d %v", len(line), line)
		}
		imu.magnetometer.X = scalemag(line[1], line[2], 1) // converts to gauss
		imu.magnetometer.Y = scalemag(line[3], line[4], 1)
		imu.magnetometer.Z = scalemag(line[5], line[6], 1)
	}

	return nil
}

// Close shuts down wit and closes imu.port.
func (imu *wit) Close() error {
	imu.logger.Debug("Closing wit motion imu")
	imu.cancelFunc()
	imu.activeBackgroundWorkers.Wait()
	imu.logger.Debug("Closed wit motion imu")
	return imu.err.Get()
}
