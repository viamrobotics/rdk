package rtk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

type i2cCorrectionSource struct {
	correctionReader io.ReadCloser // reader for rctm corrections only
	logger           golog.Logger
	bus              board.I2C
	addr             byte

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func newi2cCorrectionSource(
	ctx context.Context,
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) (correctionSource, error) {
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

	s := &i2cCorrectionSource{bus: i2cbus, addr: byte(addr), cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	return s, nil
}

// Start reads correction data from the i2c address and sends it into the correctionReader.
func (s *i2cCorrectionSource) Start(ready chan<- bool) {
	// currently not checking if rtcm message is valid, need to figure out how to integrate constant I2C byte message with rtcm3 scanner
	s.activeBackgroundWorkers.Add(1)
	defer s.activeBackgroundWorkers.Done()

	var w *io.PipeWriter
	s.correctionReader, w = io.Pipe()
	ready <- true

	// open I2C handle every time
	handle, err := s.bus.OpenHandle(s.addr)
	if err != nil {
		s.logger.Fatalf("can't open gps i2c handle: %s", err)
		return
	}

	// read from handle and pipe to correctionSource
	buffer, err := handle.Read(context.Background(), 1024)
	_, err = w.Write(buffer)
	if err != nil {
		s.logger.Fatalf("Error writing RTCM message: %s", err)
	}

	// close I2C handle
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

		// read from handle and pipe to correctionSource
		buffer, err := handle.Read(context.Background(), 1024)
		_, err = w.Write(buffer)
		if err != nil {
			s.logger.Fatalf("Error writing RTCM message: %s", err)
		}

		// close I2C handle
		err = handle.Close()
		if err != nil {
			s.logger.Debug("failed to close handle: %s", err)
			return
		}
	}
}

// GetReader returns the i2cCorrectionSource's correctionReader if it exists.
func (s *i2cCorrectionSource) GetReader() (io.ReadCloser, error) {
	if s.correctionReader == nil {
		return nil, errors.New("no stream")
	}

	return s.correctionReader, nil
}

// Close shuts down the i2cCorrectionSource.
func (s *i2cCorrectionSource) Close() error {
	s.cancelFunc()
	s.activeBackgroundWorkers.Wait()

	// close correction reader
	if s.correctionReader != nil {
		if err := s.correctionReader.Close(); err != nil {
			return err
		}
		s.correctionReader = nil
	}

	return nil
}
