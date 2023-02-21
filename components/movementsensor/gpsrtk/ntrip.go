package gpsrtk

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/edaniels/golog"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	rdkutils "go.viam.com/rdk/utils"
)

type ntripCorrectionSource struct {
	correctionReader io.ReadCloser
	info             *NtripInfo
	logger           golog.Logger
	ntripStatus      bool

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	err movementsensor.LastError
}

// NtripInfo contains the information necessary to connect to a mountpoint.
type NtripInfo struct {
	URL                string
	Username           string
	Password           string
	MountPoint         string
	Client             *ntrip.Client
	Stream             io.ReadCloser
	MaxConnectAttempts int
}

func newNtripCorrectionSource(ctx context.Context, cfg config.Component, logger golog.Logger) (correctionSource, error) {
	attr, ok := cfg.ConvertedAttributes.(*StationConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, cfg.ConvertedAttributes)
	}
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	n := &ntripCorrectionSource{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	// Init ntripInfo from attributes
	ntripInfoComp, err := newNtripInfo(attr.NtripAttrConfig, logger)
	if err != nil {
		return nil, err
	}
	n.info = ntripInfoComp

	n.logger.Debug("Returning n")
	return n, n.err.Get()
}

func newNtripInfo(cfg *NtripAttrConfig, logger golog.Logger) (*NtripInfo, error) {
	n := &NtripInfo{}

	// Init NtripInfo from attributes
	n.URL = cfg.NtripAddr
	if n.URL == "" {
		return nil, fmt.Errorf("NTRIP expected non-empty string for %q", cfg.NtripAddr)
	}
	n.Username = cfg.NtripUser
	if n.Username == "" {
		logger.Info("ntrip_username set to empty")
	}
	n.Password = cfg.NtripPass
	if n.Password == "" {
		logger.Info("ntrip_password set to empty")
	}
	n.MountPoint = cfg.NtripMountpoint
	if n.MountPoint == "" {
		logger.Info("ntrip_mountpoint set to empty")
	}
	n.MaxConnectAttempts = cfg.NtripConnectAttempts
	if n.MaxConnectAttempts == 10 {
		logger.Info("ntrip_connect_attempts using default 10")
	}

	logger.Debug("Returning n")
	return n, nil
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

	return n.err.Get()
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

	return n.err.Get()
}

// Start connects to the ntrip caster and stream and sends filtered correction data into the correctionReader.
func (n *ntripCorrectionSource) Start(ready chan<- bool) {
	n.activeBackgroundWorkers.Add(1)
	defer n.activeBackgroundWorkers.Done()
	err := n.Connect()
	if err != nil {
		n.err.Set(err)
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
		n.err.Set(err)
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
					n.err.Set(err)
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

// Reader returns the ntripCorrectionSource's correctionReader if it exists.
func (n *ntripCorrectionSource) Reader() (io.ReadCloser, error) {
	if n.correctionReader == nil {
		return nil, errors.New("no stream")
	}

	return n.correctionReader, n.err.Get()
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

	return n.err.Get()
}
