package gpsrtk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

type i2cCorrectionSource struct {
	correctionReader io.ReadCloser // reader for rctm corrections only
	logger           golog.Logger
	bus              board.I2C
	addr             byte

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	err movementsensor.LastError
}

func newI2CCorrectionSource(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (correctionSource, error) {
	attr, ok := cfg.ConvertedAttributes.(*StationConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, cfg.ConvertedAttributes)
	}
	b, err := board.FromDependencies(deps, attr.Board)
	if err != nil {
		return nil, fmt.Errorf("gps init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", attr.Board)
	}
	i2cbus, ok := localB.I2CByName(attr.I2CBus)
	if !ok {
		return nil, fmt.Errorf("gps init: failed to find i2c bus %s", attr.I2CBus)
	}
	addr := attr.I2cAddr
	if addr == -1 {
		return nil, errors.New("must specify gps i2c address")
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	s := &i2cCorrectionSource{
		bus:        i2cbus,
		addr:       byte(addr),
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	return s, s.err.Get()
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
		s.logger.Errorf("can't open gps i2c handle: %s", err)
		s.err.Set(err)
		return
	}

	// read from handle and pipe to correctionSource
	buffer, err := handle.Read(context.Background(), 1024)
	if err != nil {
		s.logger.Debug("Could not read from handle")
	}
	_, err = w.Write(buffer)
	if err != nil {
		s.logger.Errorf("Error writing RTCM message: %s", err)
		s.err.Set(err)
		return
	}

	// close I2C handle
	err = handle.Close()
	if err != nil {
		s.logger.Debug("failed to close handle: %s", err)
		s.err.Set(err)
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
			s.logger.Errorf("can't open gps i2c handle: %s", err)
			s.err.Set(err)
			return
		}

		// read from handle and pipe to correctionSource
		buffer, err := handle.Read(context.Background(), 1024)
		if err != nil {
			s.logger.Debug("Could not read from handle")
		}
		_, err = w.Write(buffer)
		if err != nil {
			s.logger.Errorf("Error writing RTCM message: %s", err)
			s.err.Set(err)
			return
		}

		// close I2C handle
		err = handle.Close()
		if err != nil {
			s.logger.Debug("failed to close handle: %s", err)
			s.err.Set(err)
			return
		}
	}
}

// Reader returns the i2cCorrectionSource's correctionReader if it exists.
func (s *i2cCorrectionSource) Reader() (io.ReadCloser, error) {
	if s.correctionReader == nil {
		return nil, errors.New("no stream")
	}

	return s.correctionReader, s.err.Get()
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

	return s.err.Get()
}

// PMTK checksums commands by XORing together each byte.
func addChk(data []byte) []byte {
	chk := checksum(data)
	newCmd := []byte("$")
	newCmd = append(newCmd, data...)
	newCmd = append(newCmd, []byte("*")...)
	newCmd = append(newCmd, chk)
	return newCmd
}

func checksum(data []byte) byte {
	var chk byte
	for _, b := range data {
		chk ^= b
	}
	return chk
}
