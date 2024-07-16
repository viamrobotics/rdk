// Package gpsutils implements necessary functions to set and return
// NTRIP information here.
package gpsutils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/de-bkg/gognss/pkg/ntrip"

	"go.viam.com/rdk/logging"
)

// NtripInfo contains the information necessary to connect to a mountpoint.
type NtripInfo struct {
	// All of these should be considered immutable.
	URL                string
	username           string
	password           string
	MountPoint         string
	MaxConnectAttempts int

	// These ones are mutable!
	Client *ntrip.Client
	Stream io.ReadCloser
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
	n.URL = cfg.NtripURL
	if n.URL == "" {
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
	n.MaxConnectAttempts = cfg.NtripConnectAttempts
	if n.MaxConnectAttempts == 0 {
		logger.Info("ntrip_connect_attempts using default 10")
		n.MaxConnectAttempts = 10
	}

	logger.Debug("Returning n")
	return n, nil
}

// ParseSourcetable gets the sourcetable and parses it.
func (n *NtripInfo) ParseSourcetable(logger logging.Logger) (*Sourcetable, error) {
	reader, err := n.Client.GetSourcetable()
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
			return nil, fmt.Errorf("%s: illegal sourcetable line: '%s'", n.URL, ln)
		}
	}

	return st, nil
}

// Connect attempts to initialize a new ntrip client. If we're unable to connect after multiple
// attempts, we return the last error.
func (n *NtripInfo) Connect(ctx context.Context, logger logging.Logger) error {
	var c *ntrip.Client
	var err error

	logger.Debug("Connecting to NTRIP caster")
	for attempts := 0; attempts < n.MaxConnectAttempts; attempts++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// This client is used in two locations in the gps rtk stack.
		// 1. when reading from the source table of a NTRIP caster
		// 2. when receiving RCTM corrections for non-vrs mount points.
		// the VRS implementation creates its own dial connection in vrs.go for receiving corrections and sending GGA messages
		c, err = ntrip.NewClient(n.URL, ntrip.Options{Username: n.username, Password: n.password})
		if err == nil { // Success!
			logger.Info("Connected to NTRIP caster")
			n.Client = c
			return nil
		}
	}

	logger.Errorf("Can't connect to NTRIP caster: %s", err)
	return err
}
