//go:build linux

// Package gpsrtkpmtk implements a gps using serial connection
package gpsrtkserial

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.viam.com/rdk/components/board/genericlinux/buses"
)

func NewCorrectionWriter(busname string, address byte) (io.ReadWriteCloser, error) {
	bus, err := buses.NewI2cBus(busname)
	if err != nil {
		return nil, err
	}
	handle, err := bus.OpenHandle(address)
	if err != nil {
		return nil, err
	}
	correctionWriter := i2cCorrectionWriter {
		bus: bus,
		handle: handle,
	}
	return &correctionWriter, nil
}

// This implements the io.ReadWriteCloser interface.
type i2cCorrectionWriter struct {
	bus    buses.I2C
	handle buses.I2CHandle
}

func (i *i2cCorrectionWriter) Read(p []byte) (int, error) {
	return 0, errors.New("unimplemented")
}

func (i *i2cCorrectionWriter) Write(p []byte) (int, error) {
	fmt.Printf("writing %d bytes to I2C\n", len(p))
	err := i.handle.Write(context.Background(), p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (i *i2cCorrectionWriter) Close() error {
	return i.handle.Close()
}
