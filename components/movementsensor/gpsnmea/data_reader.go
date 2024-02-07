package gpsnmea

import (
	"context"
	"io"
	"sync"

	"github.com/jacobsa/go-serial/serial"
)

// DataReader represents a way to get data from a GPS NMEA device. We can read data from it using
// the channel in Lines, and we can close the device when we're done.
type DataReader interface {
	Lines() chan string
	Close() error
}

// SerialDataReader implements the DataReader interface by interacting with the device over a
// serial port.
type SerialDataReader struct {
	dev                     io.ReadWriteCloser
	data                    chan string
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// NewSerialDataReader constructs a new DataReader that gets its NMEA messages over a serial port.
func NewSerialDataReader(options serial.OpenOptions) (DataReader, error) {
	dev, err := serial.Open(options)
	if err != nil {
		return nil, err
	}

	data := make(chan string)
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	reader := SerialDataReader{dev: dev, data: data, cancelCtx: cancelCtx, cancelFunc: cancelFunc}
	reader.start()

	return &reader, nil
}

func (dr *SerialDataReader) start() {
	// TODO
}

func (dr *SerialDataReader) Lines() chan string {
	return dr.data
}

func (dr *SerialDataReader) Close() error {
	dr.cancelFunc()
	dr.activeBackgroundWorkers.Wait()
	return nil
}
