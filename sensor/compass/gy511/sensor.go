package gy511

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"sync"

	"github.com/viamrobotics/robotcore/sensor/compass"
	"github.com/viamrobotics/robotcore/serial"
)

type Device struct {
	mu          sync.Mutex
	rwc         io.ReadWriteCloser
	reader      *bufio.Reader
	lastHeading *float64
}

func New(path string) (compass.Device, error) {
	rwc, err := serial.OpenDevice(path)
	if err != nil {
		return nil, err
	}
	return &Device{rwc: rwc, reader: bufio.NewReader(rwc)}, nil
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
	line, _, err := d.reader.ReadLine()
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
