package gy511

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"sync"
	"sync/atomic"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/sensor"
	"go.viam.com/robotcore/sensor/compass"
	"go.viam.com/robotcore/serial"

	movingaverage "github.com/RobinUS2/golang-moving-average"
	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

const ModelName = "gy511"

func init() {
	api.RegisterSensor(compass.DeviceType, ModelName, func(ctx context.Context, r api.Robot, config api.Component, logger golog.Logger) (sensor.Device, error) {
		return New(ctx, config.Host, logger)
	})
}

type Device struct {
	mu            sync.Mutex
	rwc           io.ReadWriteCloser
	heading       atomic.Value
	calibrating   bool
	closeCh       chan struct{}
	activeWorkers sync.WaitGroup
}

const headingWindow = 100

func New(ctx context.Context, path string, logger golog.Logger) (dev *Device, err error) {
	rwc, err := serial.OpenDevice(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = multierr.Combine(err, rwc.Close())
	}()
	d := &Device{rwc: rwc}
	if err := d.StopCalibration(ctx); err != nil {
		return nil, err
	}
	d.heading.Store(math.NaN())

	// discard serial buffer
	var discardBuf [64]byte
	//nolint
	d.rwc.Read(discardBuf[:])

	// discard first newline
	buf := bufio.NewReader(d.rwc)
	_, _, err = buf.ReadLine()
	if err != nil {
		return nil, err
	}

	d.closeCh = make(chan struct{})
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
		d.heading.Store(ma.Avg())
	}

	d.activeWorkers.Add(1)
	go func() {
		defer d.activeWorkers.Done()
		for {
			select {
			case <-d.closeCh:
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
			d.heading.Store(ma.Avg())
		}
	}()
	return d, nil
}

func (d *Device) Desc() sensor.DeviceDescription {
	return sensor.DeviceDescription{compass.DeviceType, ""}
}

func (d *Device) StartCalibration(ctx context.Context) error {
	d.mu.Lock()
	d.calibrating = true
	d.mu.Unlock()
	_, err := d.rwc.Write([]byte{'1'})
	return err
}

func (d *Device) StopCalibration(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calibrating = false
	_, err := d.rwc.Write([]byte{'0'})
	return err
}

func (d *Device) Readings(ctx context.Context) ([]interface{}, error) {
	heading, err := d.Heading(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (d *Device) Heading(ctx context.Context) (float64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.calibrating {
		return math.NaN(), nil
	}
	return d.heading.Load().(float64), nil
}

func (d *Device) Close() error {
	close(d.closeCh)
	err := d.rwc.Close()
	d.activeWorkers.Wait()
	return err
}

// RawDevice demonstrates the binary protocol used to talk to a GY511
// based on the arduino code in the directory below.
type RawDevice struct {
	mu          sync.Mutex
	calibrating bool
	heading     float64
	failAfter   int
}

func NewRawDevice() *RawDevice {
	return &RawDevice{failAfter: -1}
}

func (rd *RawDevice) SetHeading(heading float64) {
	rd.mu.Lock()
	rd.heading = heading
	rd.mu.Unlock()
}

func (rd *RawDevice) SetFailAfter(after int) {
	rd.mu.Lock()
	rd.failAfter = after
	rd.mu.Unlock()
}

func (rd *RawDevice) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, errors.New("expected read data to be non-empty")
	}
	rd.mu.Lock()
	failAfter := rd.failAfter
	if failAfter == 0 {
		rd.mu.Unlock()
		return 0, errors.New("read fail")
	}
	rd.failAfter--
	rd.mu.Unlock()
	if rd.calibrating {
		return 0, nil
	}
	rd.mu.Lock()
	heading := rd.heading
	rd.mu.Unlock()
	val := []byte(fmt.Sprintf("%0.3f\n", heading))
	copy(p, val)
	n := len(val)
	if len(p) < n {
		n = len(p)
	}
	return n, nil
}

func (rd *RawDevice) Write(p []byte) (int, error) {
	if len(p) > 1 {
		return 0, errors.New("write data must be one byte")
	}
	rd.mu.Lock()
	failAfter := rd.failAfter
	if failAfter == 0 {
		rd.mu.Unlock()
		return 0, errors.New("write fail")
	}
	rd.failAfter--
	rd.mu.Unlock()
	c := p[0]
	switch c {
	case '0':
		rd.calibrating = false
	case '1':
		rd.calibrating = true
	default:
		return 0, fmt.Errorf("unknown command on write: %q", c)
	}
	return len(p), nil
}

func (rd *RawDevice) Close() error {
	return nil
}
