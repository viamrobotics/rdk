package gpsrtk

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/edaniels/golog"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/jacobsa/go-serial/serial"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
)

type serialCorrectionSource struct {
	port   io.ReadCloser // reads all messages from port
	logger golog.Logger

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	correctionReaderMu sync.Mutex
	correctionReader   io.ReadCloser // reader for rctm corrections only

	err movementsensor.LastError
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

func newSerialCorrectionSource(conf *StationConfig, logger golog.Logger) (correctionSource, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &serialCorrectionSource{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
		err:        movementsensor.NewLastError(1, 1),
	}

	serialPath := conf.SerialCorrectionPath
	if serialPath == "" {
		return nil, fmt.Errorf("serialCorrectionSource expected non-empty string for %q", correctionPathName)
	}

	baudRate := conf.SerialCorrectionBaudRate
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

	return s, s.err.Get()
}

// Start reads correction data from the serial port and sends it into the correctionReader.
func (s *serialCorrectionSource) Start(ready chan<- bool) {
	s.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer s.activeBackgroundWorkers.Done()

		if err := s.cancelCtx.Err(); err != nil {
			return
		}

		var w io.WriteCloser
		pr, pw := io.Pipe()
		s.correctionReaderMu.Lock()
		if err := s.cancelCtx.Err(); err != nil {
			return
		}
		s.correctionReader = pipeReader{pr: pr}
		s.correctionReaderMu.Unlock()
		w = pipeWriter{pw: pw}
		select {
		case ready <- true:
		case <-s.cancelCtx.Done():
			return
		}

		// read from s.port and write rctm messages into w, discard other messages in loop
		scanner := rtcm3.NewScanner(s.port)

		for {
			select {
			case <-s.cancelCtx.Done():
				err := w.Close()
				if err != nil {
					s.logger.Errorf("Unable to close writer: %s", err)
					s.err.Set(err)
					return
				}
				return
			default:
			}

			msg, err := scanner.NextMessage()
			if err != nil {
				s.logger.Errorf("Error reading RTCM message: %s", err)
				s.err.Set(err)
				return
			}

			switch msg.(type) {
			case rtcm3.MessageUnknown:
				continue
			default:
				frame := rtcm3.EncapsulateMessage(msg)
				byteMsg := frame.Serialize()
				_, err := w.Write(byteMsg)
				if err != nil {
					s.logger.Errorf("Error writing RTCM message: %s", err)
					s.err.Set(err)
					return
				}
			}
		}
	})
}

// Reader returns the serialCorrectionSource's correctionReader if it exists.
func (s *serialCorrectionSource) Reader() (io.ReadCloser, error) {
	if s.correctionReader == nil {
		return nil, errors.New("no stream")
	}

	return s.correctionReader, s.err.Get()
}

// Close shuts down the serialCorrectionSource and closes s.port.
func (s *serialCorrectionSource) Close(ctx context.Context) error {
	s.correctionReaderMu.Lock()
	s.cancelFunc()

	// close port reader
	if s.port != nil {
		if err := s.port.Close(); err != nil {
			s.correctionReaderMu.Unlock()
			return err
		}
		s.port = nil
	}

	// close correction reader
	if s.correctionReader != nil {
		if err := s.correctionReader.Close(); err != nil {
			s.correctionReaderMu.Unlock()
			return err
		}
		s.correctionReader = nil
	}

	s.correctionReaderMu.Unlock()
	s.activeBackgroundWorkers.Wait()

	return s.err.Get()
}
