package rtk

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/edaniels/golog"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
)

type serialCorrectionSource struct {
	correctionReader io.ReadCloser // reader for rctm corrections only
	port             io.ReadCloser // reads all messages from port
	logger           golog.Logger

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type pipeReader struct {
	pr *io.PipeReader
}

func (r pipeReader) Read(p []byte) (int, error) {
	return r.pr.Read(p)
}

func (r pipeReader) Close() error {
	return r.pr.Close()
}

type pipeWriter struct {
	pw *io.PipeWriter
}

func (r pipeWriter) Write(p []byte) (int, error) {
	return r.pw.Write(p)
}

func (r pipeWriter) Close() error {
	return r.pw.Close()
}

const (
	correctionPathName = "correction_path"
	baudRateName       = "correction_baud"
)

func newSerialCorrectionSource(ctx context.Context, config config.Component, logger golog.Logger) (correctionSource, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	s := &serialCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	serialPath := config.Attributes.String(correctionPathName)
	if serialPath == "" {
		return nil, fmt.Errorf("serialCorrectionSource expected non-empty string for %q", correctionPathName)
	}

	baudRate := config.Attributes.Int(baudRateName, 0)
	if baudRate == 0 {
		baudRate = 9600
		s.logger.Info("SerialCorrectionSource: correction_baud using default 9600")
	}

	options := serial.OpenOptions{
		PortName:        serialPath,
		BaudRate:        uint(baudRate),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	var err error
	s.port, err = serial.Open(options)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Start reads correction data from the serial port and sends it into the correctionReader.
func (s *serialCorrectionSource) Start(ready chan<- bool) {
	s.activeBackgroundWorkers.Add(1)
	defer s.activeBackgroundWorkers.Done()

	var w io.WriteCloser
	pr, pw := io.Pipe()
	s.correctionReader = pipeReader{pr: pr}
	w = pipeWriter{pw: pw}
	ready <- true

	// read from s.port and write rctm messages into w, discard other messages in loop
	scanner := rtcm3.NewScanner(s.port)

	for {
		select {
		case <-s.cancelCtx.Done():
			err := w.Close()
			if err != nil {
				s.logger.Fatalf("Unable to close writer: %s", err)
			}
			return
		default:
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			s.logger.Fatalf("Error reading RTCM message: %s", err)
		}

		switch msg.(type) {
		case rtcm3.MessageUnknown:
			continue
		default:
			frame := rtcm3.EncapsulateMessage(msg)
			byteMsg := frame.Serialize()
			_, err := w.Write(byteMsg)
			if err != nil {
				s.logger.Fatalf("Error writing RTCM message: %s", err)
			}
		}
	}
}

// GetReader returns the serialCorrectionSource's correctionReader if it exists.
func (s *serialCorrectionSource) GetReader() (io.ReadCloser, error) {
	if s.correctionReader == nil {
		return nil, errors.New("no stream")
	}

	return s.correctionReader, nil
}

// Close shuts down the serialCorrectionSource and closes s.port.
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
