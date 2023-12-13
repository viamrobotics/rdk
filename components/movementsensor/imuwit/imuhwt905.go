// Package imuwit implements wit imu
package imuwit

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
	"time"

	slib "github.com/jacobsa/go-serial/serial"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var model905 = resource.DefaultModelFamily.WithModel("imu-wit-hwt905")

func init() {
	resource.RegisterComponent(movementsensor.API, model905, resource.Registration[movementsensor.MovementSensor, *Config]{
		Constructor: newWit905,
	})
}

// newWit creates a new Wit IMU.
func newWit905(
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
		baudRate:   newConf.BaudRate,
		serialPath: newConf.Port,
	}

	options := slib.OpenOptions{
		PortName:        i.serialPath,
		BaudRate:        i.baudRate,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}
	if err := i.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	logger.Debugf("initializing wit serial connection with parameters: %+v", options)
	i.port, err = slib.Open(options)
	if err != nil {
		return nil, err
	}

	portReader := bufio.NewReader(i.port)
	i.start905UpdateLoop(portReader, logger)

	return &i, nil
}

func (imu *wit) start905UpdateLoop(portReader *bufio.Reader, logger logging.Logger) {
	imu.hasMagnetometer = false
	imu.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer imu.activeBackgroundWorkers.Done()

		for {
			if imu.cancelCtx.Err() != nil {
				return
			}

			select {
			case <-imu.cancelCtx.Done():
				return
			case <-time.After(10 * time.Second):
				logger.Warnf("ReadString timeout exceeded")
				return
			default:
				line, err := readWithTimeout(portReader, 'U')
				if err != nil {
					logger.Error(err)
					continue
				}

				switch {
				case len(line) != 11:
					imu.numBadReadings++
				default:
					imu.err.Set(imu.parseWIT(line))
				}
			}
		}
	})
}

// readWithTimeout tries to read from the buffer until the delimiter is found or timeout occurs.
func readWithTimeout(r *bufio.Reader, delim byte) (string, error) {
	lineChan := make(chan string)
	errChan := make(chan error)

	go func() {
		line, err := r.ReadString(delim)
		if err != nil {
			errChan <- err
			return
		}
		lineChan <- line
	}()

	select {
	case line := <-lineChan:
		return line, nil
	case err := <-errChan:
		return "", err
	case <-time.After(10 * time.Second):
		return "", fmt.Errorf("timeout exceeded while reading from serial port")
	}
}
