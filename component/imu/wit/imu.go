// Package wit implements a wit IMU.
package wit

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	slib "github.com/jacobsa/go-serial/serial"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

const model = "wit"

func init() {
	registry.RegisterComponent(imu.Subtype, model, registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewWit(deps, config, logger)
		},
	})
}

type wit struct {
	angularVelocity spatialmath.AngularVelocity
	orientation     spatialmath.EulerAngles
	acceleration    r3.Vector
	magnetometer    r3.Vector
	lastError       error

	mu sync.Mutex

	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	generic.Unimplemented
}

// ReadAngularVelocity returns Angular velocity from the gyroscope in deg_per_sec.
func (i *wit) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.angularVelocity, i.lastError
}

// Read Orientatijn returns gyroscope orientation in degrees.
func (i *wit) ReadOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return &i.orientation, i.lastError
}

// ReadAcceleration returns accelerometer acceleration in mm_per_sec_per_sec.
func (i *wit) ReadAcceleration(ctx context.Context) (r3.Vector, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.acceleration, i.lastError
}

// ReadMagnetometer returns magnetic field in gauss.
func (i *wit) ReadMagnetometer(ctx context.Context) (r3.Vector, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.magnetometer, i.lastError
}

func (i *wit) GetReadings(ctx context.Context) ([]interface{}, error) {
	return imu.GetReadings(ctx, i)
}

// NewWit creates a new Wit IMU.
func NewWit(deps registry.Dependencies, config config.Component, logger golog.Logger) (imu.IMU, error) {
	options := slib.OpenOptions{
		BaudRate:        9600,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	options.PortName = config.Attributes.String("port")
	if options.PortName == "" {
		return nil, errors.New("wit imu needs a port")
	}

	port, err := slib.Open(options)
	if err != nil {
		return nil, err
	}

	portReader := bufio.NewReader(port)

	var i wit

	var ctx context.Context
	ctx, i.cancelFunc = context.WithCancel(context.Background())
	i.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer utils.UncheckedErrorFunc(port.Close)
		defer i.activeBackgroundWorkers.Done()

		for {
			if ctx.Err() != nil {
				return
			}

			line, err := portReader.ReadString('U')

			func() {
				i.mu.Lock()
				defer i.mu.Unlock()

				if err != nil {
					i.lastError = err
				} else {
					i.lastError = i.parseWIT(line)
				}
			}()
		}
	})
	return &i, nil
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

func (i *wit) parseWIT(line string) error {
	if line[0] == 0x52 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu angularVelocity %d %v", len(line), line)
		}
		i.angularVelocity.X = scale(line[1], line[2], 2000)
		i.angularVelocity.Y = scale(line[3], line[4], 2000)
		i.angularVelocity.Z = scale(line[5], line[6], 2000)
	}

	if line[0] == 0x53 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu orientation %d %v", len(line), line)
		}

		i.orientation.Roll = rutils.DegToRad(scale(line[1], line[2], 180))
		i.orientation.Pitch = rutils.DegToRad(scale(line[3], line[4], 180))
		i.orientation.Yaw = rutils.DegToRad(scale(line[5], line[6], 180))
	}

	if line[0] == 0x51 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu acceleration %d %v", len(line), line)
		}
		i.acceleration.X = scale(line[1], line[2], 16) * 9806.65 // converts of mm_per_sec_per_sec in NYC
		i.acceleration.Y = scale(line[3], line[4], 16) * 9806.65
		i.acceleration.Z = scale(line[5], line[6], 16) * 9806.65
	}

	if line[0] == 0x54 {
		if len(line) < 7 {
			return fmt.Errorf("line is wrong for imu magnetometer %d %v", len(line), line)
		}
		i.magnetometer.X = scalemag(line[1], line[2], 1) * 0.01 // converts to gauss
		i.magnetometer.Y = scalemag(line[3], line[4], 1) * 0.01
		i.magnetometer.Z = scalemag(line[5], line[6], 1) * 0.01
	}

	return nil
}

func (i *wit) Close() {
	i.cancelFunc()
	i.activeBackgroundWorkers.Wait()
}
