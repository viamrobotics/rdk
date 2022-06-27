package rtk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"errors"

	"github.com/adrianmo/go-nmea"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
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

func newSerialCorrectionSource(ctx context.Context, config config.Component, logger golog.Logger) (serialCorrectionSource, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	s := &serialCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	serialPath = config.Attributes.String(correctionPortName)
	if serialPath == "" {
		return nil, fmt.Errorf("serialCorrectionSource expected non-empty string for %q", correctionPortName)
	}

	s.port, err := serial.Open(serialPath)

	return s, nil
}

func (s *serialCorrectionSource) Start(ctx context.Context, ready chan<- bool) {
	ntrip.correctionReader, w := io.Pipe()
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
			g.logger.Fatalf("Error reading RTCM message: %s", err)
		}
		fmt.Println(msg.Number())

		n, err := w.Write(msg.Serialize())
		if err != nil {
			g.logger.Fatalf("Error writing RTCM message: %s", err)
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