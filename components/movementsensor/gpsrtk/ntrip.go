package gpsrtk

import (
	"fmt"
	"io"

	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/edaniels/golog"
)

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

func newNtripInfo(cfg *NtripConfig, logger golog.Logger) (*NtripInfo, error) {
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
