package gpsrtk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
)

type i2cCorrectionSource struct {
	correctionReader io.ReadCloser // reader for rctm corrections only
	logger           golog.Logger
	bus              board.I2C
	addr             byte

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	mu                      sync.Mutex

	err movementsensor.LastError
}

func newI2CCorrectionSource(
	deps resource.Dependencies,
	conf *StationConfig,
	logger golog.Logger,
) (correctionSource, error) {
	b, err := board.FromDependencies(deps, conf.Board)
	if err != nil {
		return nil, fmt.Errorf("gps init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, fmt.Errorf("board %s is not local", conf.Board)
	}
	i2cbus, ok := localB.I2CByName(conf.I2CBus)
	if !ok {
		return nil, fmt.Errorf("gps init: failed to find i2c bus %s", conf.I2CBus)
	}
	addr := conf.I2cAddr
	if addr == -1 {
		return nil, errors.New("must specify gps i2c address")
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &i2cCorrectionSource{
		bus:        i2cbus,
		addr:       byte(addr),
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
		// Overloaded boards can have flaky I2C busses. Only report errors if at least 5 of the
		// last 10 attempts have failed.
		err: movementsensor.NewLastError(10, 5),
	}

	return s, s.err.Get()
}

// Start reads correction data from the i2c address and sends it into the correctionReader.
func (s *i2cCorrectionSource) Start(ready chan<- bool) {
	s.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer s.activeBackgroundWorkers.Done()

		// currently not checking if rtcm message is valid, need to figure out how to integrate constant I2C byte message with rtcm3 scanner

		s.mu.Lock()
		if err := s.cancelCtx.Err(); err != nil {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		var w *io.PipeWriter
		s.correctionReader, w = io.Pipe()
		select {
		case ready <- true:
		case <-s.cancelCtx.Done():
			return
		}

		// open I2C handle every time
		handle, err := s.bus.OpenHandle(s.addr)
		// Record the error value no matter what. If it's nil, this will prevent us from reporting
		// ephemeral errors later.
		s.err.Set(err)
		if err != nil {
			s.logger.Errorf("can't open gps i2c handle: %s", err)
			return
		}

		// read from handle and pipe to correctionSource
		buffer, err := handle.Read(context.Background(), 1024)
		if err != nil {
			s.logger.Debug("Could not read from handle")
		}
		_, err = w.Write(buffer)
		s.err.Set(err)
		if err != nil {
			s.logger.Errorf("Error writing RTCM message: %s", err)
			return
		}

		// close I2C handle
		err = handle.Close()
		s.err.Set(err)
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
			s.err.Set(err)
			if err != nil {
				s.logger.Errorf("can't open gps i2c handle: %s", err)
				return
			}

			// read from handle and pipe to correctionSource
			buffer, err := handle.Read(context.Background(), 1024)
			if err != nil {
				s.logger.Debug("Could not read from handle")
			}
			_, err = w.Write(buffer)
			s.err.Set(err)
			if err != nil {
				s.logger.Errorf("Error writing RTCM message: %s", err)
				return
			}

			// close I2C handle
			err = handle.Close()
			s.err.Set(err)
			if err != nil {
				s.logger.Debug("failed to close handle: %s", err)
				return
			}
		}
	})
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
	s.mu.Lock()
	s.cancelFunc()
	s.mu.Unlock()
	s.activeBackgroundWorkers.Wait()

	// close correction reader
	if s.correctionReader != nil {
		if err := s.correctionReader.Close(); err != nil {
			return err
		}
		s.correctionReader = nil
	}

	if err := s.err.Get(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
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
