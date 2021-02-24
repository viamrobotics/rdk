package gy511

import (
	"bufio"
	"context"
	"io"
	"math"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/viamrobotics/robotcore/sensor/compass"
	"github.com/viamrobotics/robotcore/serial"

	movingaverage "github.com/RobinUS2/golang-moving-average"
	"github.com/edaniels/golog"
)

type Device struct {
	mu          sync.Mutex
	rwc         io.ReadWriteCloser
	heading     atomic.Value
	calibrating bool
	closeCh     chan struct{}
}

const headingWindow = 100

func New(path string) (compass.Device, error) {
	rwc, err := serial.OpenDevice(path)
	if err != nil {
		return nil, err
	}
	d := &Device{rwc: rwc}
	if err := d.StopCalibration(); err != nil {
		if err := rwc.Close(); err != nil {
			return nil, err
		}
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
	go func() {
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
		for {
			select {
			case <-d.closeCh:
				return
			default:
			}
			heading, err := readHeading()
			if err != nil {
				golog.Global.Debugw("error reading heading", "error", err)
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

func (d *Device) StartCalibration() error {
	d.mu.Lock()
	d.calibrating = true
	d.mu.Unlock()
	_, err := d.rwc.Write([]byte{'1'})
	return err
}

func (d *Device) StopCalibration() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calibrating = false
	_, err := d.rwc.Write([]byte{'0'})
	return err
}

func (d *Device) Readings(ctx context.Context) ([]interface{}, error) {
	heading, err := d.Heading()
	if err != nil {
		return nil, err
	}
	return []interface{}{heading}, nil
}

func (d *Device) Heading() (float64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.calibrating {
		return math.NaN(), nil
	}
	return d.heading.Load().(float64), nil
}

func (d *Device) Close(ctx context.Context) error {
	close(d.closeCh)
	return d.rwc.Close()
}
