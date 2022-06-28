package rtk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"errors"
	"bytes"

	"github.com/adrianmo/go-nmea"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

type I2CCorrectionSource struct {
	correctionReader    	io.ReadCloser // reader for rctm corrections only
	port					io.ReadCloser // reads all messages from port
	logger             		golog.Logger
	ntripStatus        		bool
	bus    board.I2C
	addr   byte

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func newI2CCorrectionSource(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (correctionSource, error) {
	b, err := board.FromDependencies(deps, config.Attributes.String("board"))
	if err != nil {
		return nil, fmt.Errorf("gps init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", config.Attributes.String("board"))
	}
	i2cbus, ok := localB.I2CByName(config.Attributes.String("bus"))
	if !ok {
		return nil, fmt.Errorf("gps init: failed to find i2c bus %s", config.Attributes.String("bus"))
	}
	addr := config.Attributes.Int("i2c_addr", -1)
	if addr == -1 {
		return nil, errors.New("must specify gps i2c address")
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	s := &I2CCorrectionSource{bus: i2cbus, addr: byte(addr), cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	return s, nil
}

func (s *I2CCorrectionSource) Start(ctx context.Context, ready chan<- bool) {
//currently not checking if rtcm message is valid, need to figure out how to integrate constant I2C byte message with rtcm3 scanner
	var w *io.PipeWriter
	s.correctionReader, w = io.Pipe()
	ready <- true

	// open I2C handle every time
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		s.logger.Fatalf("can't open gps i2c handle: %s", err)
		return
	}

	//read from handle and pipe to correctionSource
	buffer, err := handle.Read(context.Background(), 1024)
	_, err = w.Write(buffer)
	if err != nil {
		s.logger.Fatalf("Error writing RTCM message: %s", err)
	}

	//close I2C handle
	err = handle.Close()
	if err != nil {
		s.logger.Debug("failed to close handle: %s", err)
		return
	}

	for err == nil {
		select {
		case <-s.cancelCtx.Done():
			return
		default:
		}

		// Open I2C handle every time
		handle, err := s.bus.OpenHandle(s.addr)
		if err != nil {
			s.logger.Fatalf("can't open gps i2c handle: %s", err)
			return
		}

		//read from handle and pipe to correctionSource
		buffer, err := handle.Read(context.Background(), 1024)
		_, err = w.Write(buffer)
		if err != nil {
			s.logger.Fatalf("Error writing RTCM message: %s", err)
		}

		//close I2C handle
		err = handle.Close()
		if err != nil {
			s.logger.Debug("failed to close handle: %s", err)
			return
		}
	}
}

func (s *I2CCorrectionSource) GetReader() (io.ReadCloser, error) {
	if s.correctionReader == nil {
		return nil, errors.New("No Stream")
	}

	return s.correctionReader, nil
}

func (s *I2CCorrectionSource) Close() error {
	s.cancelFunc()
	s.activeBackgroundWorkers.Wait()

	// close port reader
	if s.port != nil {
		if err := s.port.Close(); err != nil {
			return err
		}
		s.port = nil
	}

	// close correction reader
	if s.correctionReader != nil {
		if err := s.correctionReader.Close(); err != nil {
			return err
		}
		s.correctionReader = nil
	}

	return nil
}