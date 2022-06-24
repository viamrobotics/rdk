package rtk

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"errors"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
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

func newNtripCorrectionSource(ctx context.Context, config config.Component, logger golog.Logger) (ntripCorrectionSource, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	ntrip := &ntripCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	// Init ntripInfo from attributes
	ntrip.info.url = config.Attributes.String(ntripAddrAttrName)
	if ntrip.info.url == "" {
		return nil, fmt.Errorf("NTRIP expected non-empty string for %q", ntripAddrAttrName)
	}
	ntrip.info.username = config.Attributes.String(ntripUserAttrName)
	if ntrip.info.username == "" {
		ntrip.logger.Info("ntrip_username set to empty")
	}
	ntrip.info.password = config.Attributes.String(ntripPassAttrName)
	if ntrip.info.password == "" {
		ntrip.logger.Info("ntrip_password set to empty")
	}
	ntrip.info.mountPoint = config.Attributes.String(ntripMountPointAttrName)
	if ntrip.info.mountPoint == "" {
		ntrip.logger.Info("ntrip_mountpoint set to empty")
	}
	ntrip.info.maxConnectAttempts = config.Attributes.Int(ntripConnectAttemptsName, 10)
	if ntrip.info.maxConnectAttempts == 10 {
		ntrip.logger.Info("ntrip_connect_attempts using default 10")
	}

	return ntrip, nil
}

// Connect attempts to connect to ntrip client until successful connection or timeout.
func (ntrip *ntripCorrectionSource) Connect() error {
	success := false
	attempts := 0

	var c *ntrip.Client
	var err error

	ntrip.logger.Debug("Connecting to NTRIP caster")
	for !success && attempts < ntrip.info.maxConnectAttempts {
		select {
		case <-ntrip.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		c, err = ntrip.NewClient(ntrip.info.url, ntrip.Options{Username: ntrip.info.username, Password: ntrip.info.password})
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		ntrip.logger.Errorf("Can't connect to NTRIP caster: %s", err)
		return err
	}

	ntrip.info.client = c

	ntrip.logger.Debug("Connected to NTRIP caster")

	return nil
}

// GetStream attempts to connect to ntrip stream until successful connection or timeout.
func (ntrip *ntripCorrectionSource) GetStream() error {
	success := false
	attempts := 0

	var rc io.ReadCloser
	var err error

	ntrip.logger.Debug("Getting NTRIP stream")

	for !success && attempts < ntrip.info.maxConnectAttempts {
		select {
		case <-ntrip.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		rc, err = ntrip.info.client.GetStream(ntrip.info.mountPoint)
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		ntrip.logger.Errorf("Can't connect to NTRIP stream: %s", err)
		return err
	}

	ntrip.info.stream = rc

	g.logger.Debug("Connected to stream")

	return nil
}

func (ntrip *ntripCorrectionSource) Start(ctx context.Context) {
	ntrip.activeBackgroundWorkers.Add(1)
	defer ntrip.activeBackgroundWorkers.Done()
	err := ntrip.Connect()
	if err != nil {
		return
	}

	if !ntrip.infp.client.IsCasterAlive() {
		ntrip.logger.Infof("caster %s seems to be down", ntrip.info.url)
	}

	ntrip.correctionReader, w := io.Pipe()

	err = ntrip.GetStream()
	if err != nil {
		return
	}

	r := io.TeeReader(ntrip.info.stream, w)
	scanner := rtcm3.NewScanner(r)

	ntrip.ntripStatus = true

	for ntrip.ntripStatus {
		select {
		case <-ntrip.cancelCtx.Done():
			return
		default:
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			ntrip.ntripStatus = false
			if msg == nil {
				ntrip.logger.Debug("No message... reconnecting to stream...")
				err = ntrip.GetStream()
				if err != nil {
					return
				}

				r = io.TeeReader(ntrip.info.stream, w)
				scanner = rtcm3.NewScanner(r)
				ntrip.ntripStatus = true
				continue
			}
		}
	}
}

func (ntrip *ntripCorrectionSource) GetReader() (io.ReadCloser, error) {
	if ntrip.correctionReader == nil {
		return nil, errors.New("No Stream")
	}

	return ntrip.correctionReader, nil
}

func (ntrip *ntripCorrectionSource) Close() error {
	ntrip.cancelFunc()
	ntrip.activeBackgroundWorkers.Wait()

	// close correction reader
	if ntrip.correctionReader != nil {
		if err := ntrip.correctionReader.Close(); err != nil {
			return err
		}
		ntrip.correctionReader = nil
	}

	// close ntrip client and stream
	if ntrip.info.client != nil {
		ntrip.info.client.CloseIdleConnections()
		ntrip.info.client = nil
	}

	if ntrip.info.stream != nil {
		if err := ntrip.info.stream.Close(); err != nil {
			return err
		}
		ntrip.info.stream = nil
	}

	return nil
}