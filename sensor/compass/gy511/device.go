package gy511

import (
	"bufio"
	"errors"
	"io"
	"math"
	"strconv"
	"sync"

	"github.com/viamrobotics/robotcore/sensor/compass"
	"github.com/viamrobotics/robotcore/serial"
)

type Device struct {
	mu          sync.Mutex
	rwc         io.ReadWriteCloser
	lastHeading *float64
	calibrating bool
}

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

func (d *Device) Readings() ([]interface{}, error) {
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

	// discard serial buffer
	var discardBuf [64]byte
	//nolint
	d.rwc.Read(discardBuf[:])

	// discard first newline
	buf := bufio.NewReader(d.rwc)
	_, _, err := buf.ReadLine()
	if err != nil {
		return 0, err
	}
	line, _, err := buf.ReadLine()
	if err != nil {
		return 0, err
	}
	if len(line) == 0 {
		if d.lastHeading == nil {
			return 0, errors.New("no last heading")
		}
		return *d.lastHeading, nil
	}
	heading, err := strconv.ParseFloat(string(line), 64)
	if err != nil {
		return 0, err
	}
	d.lastHeading = &heading
	return heading, nil
}

func (d *Device) Close() error {
	return d.rwc.Close()
}
