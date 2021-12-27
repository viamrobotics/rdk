// Package gy511 implements a GY511 based compass.
package gy511

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"sync"
	"sync/atomic"

	movingaverage "github.com/RobinUS2/golang-moving-average"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/sensor/compass"
	"go.viam.com/rdk/serial"
)

// ModelName is used to register the sensor to a model name.
const ModelName = "gy511"

// init registers the gy511 compass type.
func init() {
	registry.RegisterSensor(
		compass.Type,
		ModelName,
		registry.Sensor{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (sensor.Sensor, error) {
			return New(ctx, config.Host, logger)
		}})
}

// GY511 represents a gy511 compass.
type GY511 struct {
	rwc           io.ReadWriteCloser
	heading       atomic.Value // float64
	calibrating   uint32
	closeCh       chan struct{}
	activeWorkers *sync.WaitGroup
}

const headingWindow = 100

// New returns a new gy511 compass that communicates over serial on the given path.
func New(ctx context.Context, path string, logger golog.Logger) (dev *GY511, err error) {
	rwc, err := serial.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = multierr.Combine(err, rwc.Close())
	}()
	gy := &GY511{rwc: rwc, activeWorkers: &sync.WaitGroup{}}
	if err := gy.StopCalibration(ctx); err != nil {
		return nil, err
	}
	gy.heading.Store(math.NaN())

	// discard serial buffer
	var discardBuf [64]byte
	//nolint
	gy.rwc.Read(discardBuf[:])

	// discard first newline
	buf := bufio.NewReader(gy.rwc)
	_, _, err = buf.ReadLine()
	if err != nil {
		return nil, err
	}

	gy.closeCh = make(chan struct{})
	ma := movingaverage.New(headingWindow)
	readHeading := func() (float64, error) {
		line, _, err := buf.ReadLine()
		if err != nil {
			return math.NaN(), err
		}
		if len(line) == 0 {
			return math.NaN(), nil
		}
		return strconv.ParseFloat(string(line), 64)
	}
	heading, err := readHeading()
	if err != nil {
		return nil, err
	}
	if !math.IsNaN(heading) {
		ma.Add(heading)
		gy.heading.Store(ma.Avg())
	}

	gy.activeWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			select {
			case <-gy.closeCh:
				return
			default:
			}
			heading, err := readHeading()
			if err != nil {
				logger.Debugw("error reading heading", "error", err)
			}

			if math.IsNaN(heading) {
				continue
			}
			ma.Add(heading)
			gy.heading.Store(ma.Avg())
		}
	}, gy.activeWorkers.Done)
	return gy, nil
}

// Desc returns that this is a compass.
func (gy *GY511) Desc() sensor.Description {
	return sensor.Description{compass.Type, ""}
}

// StartCalibration asks the device to start calibrating.
func (gy *GY511) StartCalibration(ctx context.Context) error {
	atomic.StoreUint32(&gy.calibrating, 1)
	_, err := gy.rwc.Write([]byte{'1'})
	return err
}

// StopCalibration asks the device to stop calibrating.
func (gy *GY511) StopCalibration(ctx context.Context) error {
	atomic.StoreUint32(&gy.calibrating, 0)
	_, err := gy.rwc.Write([]byte{'0'})
	return err
}

// Readings returns the current yaw measurement.
func (gy *GY511) Readings(ctx context.Context) ([]interface{}, error) {
	heading, err := gy.Heading(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

// Heading returns the current yaw measurement.
func (gy *GY511) Heading(ctx context.Context) (float64, error) {
	if atomic.LoadUint32(&gy.calibrating) == 1 {
		return math.NaN(), nil
	}
	heading, ok := gy.heading.Load().(float64)
	if !ok {
		return math.NaN(), nil
	}
	return heading, nil
}

// Close terminates the serial connection.
func (gy *GY511) Close() error {
	close(gy.closeCh)
	err := gy.rwc.Close()
	gy.activeWorkers.Wait()
	return err
}

// RawGY511 demonstrates the binary protocol used to talk to a GY511
// based on the arduino code in the directory below.
type RawGY511 struct {
	calibrating uint32
	heading     atomic.Value // float64
	failAfter   int32
}

// NewRawGY511 returns a mocked representation of a serial based GY511.
func NewRawGY511() *RawGY511 {
	return &RawGY511{failAfter: -1}
}

// SetHeading sets the heading to return on reads.
func (rgy *RawGY511) SetHeading(heading float64) {
	rgy.heading.Store(heading)
}

// SetFailAfter sets after how many reads it should start erroring out.
func (rgy *RawGY511) SetFailAfter(after int) {
	atomic.StoreInt32(&rgy.failAfter, int32(after))
}

// Read returns data based on the current state of the machine.
func (rgy *RawGY511) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, errors.New("expected read data to be non-empty")
	}
	failAfter := atomic.LoadInt32(&rgy.failAfter)
	if failAfter == 0 {
		return 0, errors.New("read fail")
	}
	atomic.AddInt32(&rgy.failAfter, -1)
	if atomic.LoadUint32(&rgy.calibrating) == 1 {
		return 0, nil
	}
	heading, ok := rgy.heading.Load().(float64)
	if !ok {
		return 0, nil
	}
	val := []byte(fmt.Sprintf("%0.3f\n", heading))
	copy(p, val)
	n := len(val)
	if len(p) < n {
		n = len(p)
	}
	return n, nil
}

// Write accepts input to change the state of the machine.
func (rgy *RawGY511) Write(p []byte) (int, error) {
	if len(p) > 1 {
		return 0, errors.New("write data must be one byte")
	}
	failAfter := atomic.LoadInt32(&rgy.failAfter)
	if failAfter == 0 {
		return 0, errors.New("write fail")
	}
	atomic.AddInt32(&rgy.failAfter, -1)
	c := p[0]
	switch c {
	case '0':
		atomic.StoreUint32(&rgy.calibrating, 0)
	case '1':
		atomic.StoreUint32(&rgy.calibrating, 1)
	default:
		return 0, errors.Errorf("unknown command on write: %q", c)
	}
	return len(p), nil
}

// Close does nothing.
func (rgy *RawGY511) Close() error {
	return nil
}
