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
	i.start905UpdateLoop(portReader, logger)

	return &i, nil
}

func (imu *wit) start905UpdateLoop(portReader *bufio.Reader, logger logging.Logger) {
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
