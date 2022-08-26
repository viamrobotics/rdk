package rtk

import (
	"context"
	"io"
	"sync"

	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/edaniels/golog"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/movementsensor/nmea"
	"go.viam.com/rdk/config"
)

type ntripCorrectionSource struct {
	correctionReader io.ReadCloser
	info             *nmea.NtripInfo
	logger           golog.Logger
	ntripStatus      bool

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	errMu     sync.Mutex
	lastError error
}

func newNtripCorrectionSource(ctx context.Context, config config.Component, logger golog.Logger) (correctionSource, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	n := &ntripCorrectionSource{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	// Init ntripInfo from attributes
	ntripInfoComp, err := nmea.NewNtripInfo(ctx, config, logger)
	if err != nil {
		return nil, err
	}
	n.info = ntripInfoComp

	n.logger.Debug("Returning n")
	return n, n.lastError
}

// Connect attempts to connect to ntrip client until successful connection or timeout.
func (n *ntripCorrectionSource) Connect() error {
	success := false
	attempts := 0

	var c *ntrip.Client
	var err error

	n.logger.Debug("Connecting to NTRIP caster")
	for !success && attempts < n.info.MaxConnectAttempts {
		select {
		case <-n.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		c, err = ntrip.NewClient(n.info.URL, ntrip.Options{Username: n.info.Username, Password: n.info.Password})
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		n.logger.Errorf("Can't connect to NTRIP caster: %s", err)
		return err
	}

	n.info.Client = c

	n.logger.Debug("Connected to NTRIP caster")

	return n.lastError
}

// GetStream attempts to connect to ntrip stream until successful connection or timeout.
func (n *ntripCorrectionSource) GetStream() error {
	success := false
	attempts := 0

	var rc io.ReadCloser
	var err error

	n.logger.Debug("Getting NTRIP stream")

	for !success && attempts < n.info.MaxConnectAttempts {
		select {
		case <-n.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		rc, err = n.info.Client.GetStream(n.info.MountPoint)
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		n.logger.Errorf("Can't connect to NTRIP stream: %s", err)
		return err
	}

	n.info.Stream = rc

	n.logger.Debug("Connected to stream")

	return n.lastError
}

func (n *ntripCorrectionSource) setLastError(err error) {
	n.errMu.Lock()
	defer n.errMu.Unlock()

	n.lastError = err
}

// Start connects to the ntrip caster and stream and sends filtered correction data into the correctionReader.
func (n *ntripCorrectionSource) Start(ready chan<- bool) {
	n.activeBackgroundWorkers.Add(1)
	defer n.activeBackgroundWorkers.Done()
	err := n.Connect()
	if err != nil {
		n.setLastError(err)
		return
	}

	if !n.info.Client.IsCasterAlive() {
		n.logger.Infof("caster %s seems to be down", n.info.URL)
	}

	var w io.Writer
	n.correctionReader, w = io.Pipe()
	ready <- true

	err = n.GetStream()
	if err != nil {
		n.setLastError(err)
		return
	}

	r := io.TeeReader(n.info.Stream, w)
	scanner := rtcm3.NewScanner(r)

	n.ntripStatus = true

	for n.ntripStatus {
		select {
		case <-n.cancelCtx.Done():
			return
		default:
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			n.ntripStatus = false
			if msg == nil {
				n.logger.Debug("No message... reconnecting to stream...")
				err = n.GetStream()
				if err != nil {
					n.setLastError(err)
					return
				}

				r = io.TeeReader(n.info.Stream, w)
				scanner = rtcm3.NewScanner(r)
				n.ntripStatus = true
				continue
			}
		}
	}
}

// GetReader returns the ntripCorrectionSource's correctionReader if it exists.
func (n *ntripCorrectionSource) GetReader() (io.ReadCloser, error) {
	if n.correctionReader == nil {
		return nil, errors.New("no stream")
	}

	return n.correctionReader, n.lastError
}

// Close shuts down the ntripCorrectionSource and closes all connections to the caster.
func (n *ntripCorrectionSource) Close() error {
	n.logger.Debug("Closing ntrip client")
	n.cancelFunc()
	n.activeBackgroundWorkers.Wait()

	// close correction reader
	if n.correctionReader != nil {
		if err := n.correctionReader.Close(); err != nil {
			return err
		}
		n.correctionReader = nil
	}

	// close ntrip client and stream
	if n.info.Client != nil {
		n.info.Client.CloseIdleConnections()
		n.info.Client = nil
	}

	if n.info.Stream != nil {
		if err := n.info.Stream.Close(); err != nil {
			return err
		}
		n.info.Stream = nil
	}

	return n.lastError
}
