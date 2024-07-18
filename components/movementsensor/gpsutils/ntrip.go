// Package gpsutils implements necessary functions to set and return
// NTRIP information here.
package gpsutils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/de-bkg/gognss/pkg/ntrip"

	"go.viam.com/rdk/logging"
)

// NtripInfo contains the information necessary to connect to a mountpoint.
type NtripInfo struct {
	// All of these should be considered immutable.
	url                string
	username           string
	password           string
	MountPoint         string
	maxConnectAttempts int

	// These ones are mutable!
	mu     sync.Mutex
	client *ntrip.Client
	stream io.ReadCloser
}

// NtripConfig is used for converting attributes for a correction source.
type NtripConfig struct {
	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
}

// NewNtripInfo function validates and sets NtripConfig arributes and returns NtripInfo.
func NewNtripInfo(cfg *NtripConfig, logger logging.Logger) (*NtripInfo, error) {
	n := &NtripInfo{}

	// Init NtripInfo from attributes
	n.url = cfg.NtripURL
	if n.url == "" {
		return nil, fmt.Errorf("NTRIP expected non-empty string for %q", cfg.NtripURL)
	}
	n.username = cfg.NtripUser
	if n.username == "" {
		logger.Info("ntrip_username set to empty")
	}
	n.password = cfg.NtripPass
	if n.password == "" {
		logger.Info("ntrip_password set to empty")
	}
	n.MountPoint = cfg.NtripMountpoint
	if n.MountPoint == "" {
		logger.Info("ntrip_mountpoint set to empty")
	}
	n.maxConnectAttempts = cfg.NtripConnectAttempts
	if n.maxConnectAttempts == 0 {
		logger.Info("ntrip_connect_attempts using default 10")
		n.maxConnectAttempts = 10
	}

	logger.Debug("Returning n")
	return n, nil
}

// ParseSourcetable gets the sourcetable and parses it.
func (n *NtripInfo) ParseSourcetable(logger logging.Logger) (*Sourcetable, error) {
	reader, err := n.client.GetSourcetable()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := reader.Close(); err != nil {
			logger.Errorf("Error closing reader:", err)
		}
	}()

	st := &Sourcetable{}
	st.Streams = make([]Stream, 0, streamSize)
	scanner := bufio.NewScanner(reader)

Loop:
	for scanner.Scan() {
		ln := scanner.Text()

		// Check if the line is a comment.
		if strings.HasPrefix(ln, "#") || strings.HasPrefix(ln, "*") {
			continue
		}
		fields := strings.Split(ln, ";")
		switch fields[0] {
		case "CAS", "NET":
			continue
		case "STR":
			if fields[mp] == n.MountPoint {
				str, err := parseStream(ln) // Defined in source_table.go
				if err != nil {
					return nil, fmt.Errorf("error while parsing stream: %w", err)
				}
				st.Streams = append(st.Streams, str)
			}
		default:
			if strings.HasPrefix(fields[0], "END") {
				logger.Debug("Reached the end of SourceTable")
				break Loop
			}
			return nil, fmt.Errorf("%s: illegal sourcetable line: '%s'", n.url, ln)
		}
	}

	return st, nil
}

// Connect attempts to initialize a new ntrip client. If we're unable to connect after multiple
// attempts, we return the last error.
func (n *NtripInfo) Connect(ctx context.Context, logger logging.Logger) error {
	err := n.createConnection(ctx, logger)
	if err != nil {
		return err
	}

	return n.waitUntilCasterIsLive(logger)
}

// createConnection is a helper function called from within Connect(). It initializes n.client by
// connecting to the NTRIP caster.
func (n *NtripInfo) createConnection(ctx context.Context, logger logging.Logger) error {
	var c *ntrip.Client
	var err error

	logger.Debug("Connecting to NTRIP caster")
	for attempts := 0; attempts < n.maxConnectAttempts; attempts++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// This client is used in two locations in the gps rtk stack.
		// 1. when reading from the source table of a NTRIP caster
		// 2. when receiving RCTM corrections for non-vrs mount points.
		// the VRS implementation creates its own dial connection in vrs.go for receiving corrections and sending GGA messages
		c, err = ntrip.NewClient(n.url, ntrip.Options{Username: n.username, Password: n.password})
		if err == nil { // Success!
			logger.Info("Connected to NTRIP caster")
			n.client = c
			return nil
		}
	}

	logger.Errorf("Can't connect to NTRIP caster: %s", err)
	return err
}

// waitUntilCasterIsLive returns when n.client.IsCasterAlive() is true, or after 5 failed attempts.
// This is a helper function intended only to be called from within Connect().
func (n *NtripInfo) waitUntilCasterIsLive(logger logging.Logger) error {
	if n.client.IsCasterAlive() {
		return nil
	}

	logger.Infof("caster %s seems to be down, retrying", n.url)
	// We will try to connect to the caster five times if it's down.
	for attempts := 0; attempts < 5; attempts++ {
		if n.client.IsCasterAlive() {
			return nil
		}
		logger.Debugf("attempt(s) to connect to caster: %v ", attempts)
	}
	return fmt.Errorf("caster %s is down", n.url)
}

// GetStreamFromMountPoint attempts to connect to the NTRIP stream, store it in n.stream, and
// return it. We give up and return an error after n.maxConnectAttempts unsuccessful tries.
func (n *NtripInfo) GetStreamFromMountPoint(
	cancelCtx context.Context,
	logger logging.Logger,
) (io.ReadCloser, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Set the Timeout to 0 on the http client to prevent the ntrip stream from canceling itself.
	// ntrip.NewClient() defaults sets this value to 15 seconds, which causes us to disconnect
	// the ntrip stream and require a reconnection. Additionally, this should be tested with other
	// CORS.
	n.client.Timeout = 0

	var rc io.ReadCloser
	var err error

	logger.Debug("Getting NTRIP stream")

	for attempts := 0; attempts < n.maxConnectAttempts; attempts++ {
		logger.Debugf("unable to get stream after %d attempts, trying again...", attempts)
		select {
		case <-cancelCtx.Done():
			return nil, errors.New("Canceled")
		default:
		}

		rc, err = n.client.GetStream(n.MountPoint)
		if err == nil {
			logger.Debug("Connected to stream")
			n.stream = rc
			return rc, nil
		}
	}

	logger.Errorf("Can't connect to NTRIP stream: %s", err)
	return nil, err
}

// Close shuts down all connections to the NTRIP caster.
func (n *NtripInfo) Close(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	var err error
	if n.client != nil {
		n.client.CloseIdleConnections()
		n.client = nil
	}
	if n.stream != nil {
		err = n.stream.Close()
		n.stream = nil
	}
	return err
}
