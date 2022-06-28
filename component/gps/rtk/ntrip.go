package rtk

import (
	"context"
	"fmt"
	"io"
	"sync"
	"errors"

	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/edaniels/golog"

	"go.viam.com/rdk/config"
)

type ntripCorrectionSource struct {
	correctionReader    	io.ReadCloser
	info        			ntripInfo
	logger             		golog.Logger
	ntripStatus        		bool

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type ntripInfo struct {
	url                string
	username           string
	password           string
	mountPoint         string
	client             *ntrip.Client
	stream             io.ReadCloser
	maxConnectAttempts int
}

const (
	ntripAddrAttrName          = "ntrip_addr"
	ntripUserAttrName          = "ntrip_username"
	ntripPassAttrName          = "ntrip_password"
	ntripMountPointAttrName    = "ntrip_mountpoint"
	ntripConnectAttemptsName   = "ntrip_connect_attempts"
)

func newNtripCorrectionSource(ctx context.Context, config config.Component, logger golog.Logger) (correctionSource, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	n := &ntripCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	// Init ntripInfo from attributes
	n.info.url = config.Attributes.String(ntripAddrAttrName)
	if n.info.url == "" {
		return nil, fmt.Errorf("NTRIP expected non-empty string for %q", ntripAddrAttrName)
	}
	n.info.username = config.Attributes.String(ntripUserAttrName)
	if n.info.username == "" {
		n.logger.Info("ntrip_username set to empty")
	}
	n.info.password = config.Attributes.String(ntripPassAttrName)
	if n.info.password == "" {
		n.logger.Info("ntrip_password set to empty")
	}
	n.info.mountPoint = config.Attributes.String(ntripMountPointAttrName)
	if n.info.mountPoint == "" {
		n.logger.Info("ntrip_mountpoint set to empty")
	}
	n.info.maxConnectAttempts = config.Attributes.Int(ntripConnectAttemptsName, 10)
	if n.info.maxConnectAttempts == 10 {
		n.logger.Info("ntrip_connect_attempts using default 10")
	}

	return n, nil
}

// Connect attempts to connect to ntrip client until successful connection or timeout.
func (n *ntripCorrectionSource) Connect() error {
	success := false
	attempts := 0

	var c *ntrip.Client
	var err error

	n.logger.Debug("Connecting to NTRIP caster")
	for !success && attempts < n.info.maxConnectAttempts {
		select {
		case <-n.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		c, err = ntrip.NewClient(n.info.url, ntrip.Options{Username: n.info.username, Password: n.info.password})
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		n.logger.Errorf("Can't connect to NTRIP caster: %s", err)
		return err
	}

	n.info.client = c

	n.logger.Debug("Connected to NTRIP caster")

	return nil
}

// GetStream attempts to connect to ntrip stream until successful connection or timeout.
func (n *ntripCorrectionSource) GetStream() error {
	success := false
	attempts := 0

	var rc io.ReadCloser
	var err error

	n.logger.Debug("Getting NTRIP stream")

	for !success && attempts < n.info.maxConnectAttempts {
		select {
		case <-n.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		rc, err = n.info.client.GetStream(n.info.mountPoint)
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		n.logger.Errorf("Can't connect to NTRIP stream: %s", err)
		return err
	}

	n.info.stream = rc

	n.logger.Debug("Connected to stream")

	return nil
}

func (n *ntripCorrectionSource) Start(ctx context.Context, ready chan<- bool) {
	n.activeBackgroundWorkers.Add(1)
	defer n.activeBackgroundWorkers.Done()
	err := n.Connect()
	if err != nil {
		return
	}

	if !n.info.client.IsCasterAlive() {
		n.logger.Infof("caster %s seems to be down", n.info.url)
	}

	var w io.Writer
	n.correctionReader, w = io.Pipe()
	ready <- true

	err = n.GetStream()
	if err != nil {
		return
	}

	r := io.TeeReader(n.info.stream, w)
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
					return
				}

				r = io.TeeReader(n.info.stream, w)
				scanner = rtcm3.NewScanner(r)
				n.ntripStatus = true
				continue
			}
		}
	}
}

func (n *ntripCorrectionSource) GetReader() (io.ReadCloser, error) {
	if n.correctionReader == nil {
		return nil, errors.New("No Stream")
	}

	return n.correctionReader, nil
}

func (n *ntripCorrectionSource) Close() error {
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
	if n.info.client != nil {
		n.info.client.CloseIdleConnections()
		n.info.client = nil
	}

	if n.info.stream != nil {
		if err := n.info.stream.Close(); err != nil {
			return err
		}
		n.info.stream = nil
	}

	return nil
}