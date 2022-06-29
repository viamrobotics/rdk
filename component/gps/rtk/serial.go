package rtk

import (
	"context"
	"fmt"
	"io"
	"sync"
	"errors"

	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/edaniels/golog"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/config"
)

type serialCorrectionSource struct {
	correctionReader    	io.ReadCloser // reader for rctm corrections only
	port					io.ReadCloser // reads all messages from port
	logger             		golog.Logger
	ntripStatus        		bool

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

const (
	correctionPathName = "correction_path"
)

func newSerialCorrectionSource(ctx context.Context, config config.Component, logger golog.Logger) (correctionSource, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	s := &serialCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	serialPath := config.Attributes.String(correctionPathName)
	if serialPath == "" {
		return nil, fmt.Errorf("serialCorrectionSource expected non-empty string for %q", correctionPathName)
	}

	var err error
	s.port, err = serial.Open(serialPath)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *serialCorrectionSource) Start(ctx context.Context, ready chan<- bool) {
	s.activeBackgroundWorkers.Add(1)
	defer s.activeBackgroundWorkers.Done()

	var w io.Writer
	s.correctionReader, w = io.Pipe()
	ready <- true

	// read from s.port and write rctm messages into w, discard other messages in loop
	var err error
	scanner := rtcm3.NewScanner(s.port)

	for err == nil {
		select {
		case <-s.cancelCtx.Done():
			return
		default:
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			s.logger.Fatalf("Error reading RTCM message: %s", err)
		}
		fmt.Println(msg.Number())

		_, err = w.Write(msg.Serialize())
		if err != nil {
			s.logger.Fatalf("Error writing RTCM message: %s", err)
		}
	}
}

func (s *serialCorrectionSource) GetReader() (io.ReadCloser, error) {
	if s.correctionReader == nil {
		return nil, errors.New("No Stream")
	}

	return s.correctionReader, nil
}

func (s *serialCorrectionSource) Close() error {
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