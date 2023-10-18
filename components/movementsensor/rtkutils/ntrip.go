package rtkutils

import (
	"fmt"
	"io"

	"github.com/de-bkg/gognss/pkg/ntrip"

	"go.viam.com/rdk/logging"
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

// NtripConfig is used for converting attributes for a correction source.
type NtripConfig struct {
	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
}

// NewNtripInfo function validates and sets NtripConfig arributes and returns NtripInfo.
func NewNtripInfo(cfg *NtripConfig, logger logging.Logger) (*NtripInfo, error) {
	n := &NtripInfo{}

	// Init NtripInfo from attributes
	n.URL = cfg.NtripURL
	if n.URL == "" {
		return nil, fmt.Errorf("NTRIP expected non-empty string for %q", cfg.NtripURL)
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
		fmt.Printf("n is ->%v<-, cfg is ->%v<-, connect attempts is %v\n",n, cfg, cfg.NtripConnectAttempts)
		logger.Info("ntrip_connect_attempts using default 10")
	}

	logger.Debug("Returning n")
	return n, nil
}
