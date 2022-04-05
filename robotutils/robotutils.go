// Package robotutils are a collection of util methods for creating and running robots in rdk
package robotutils

import (
	"context"
	"crypto/tls"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/web"
	rutils "go.viam.com/rdk/utils"
)

// TLSConfig stores the TLS config for the robot.
type TLSConfig struct {
	*tls.Config
	certMu  sync.Mutex
	tlsCert *tls.Certificate
}

// NewTLSConfig creates a new tls config.
func NewTLSConfig(cfg *config.Config) *TLSConfig {
	tlsCfg := &TLSConfig{}
	var tlsConfig *tls.Config
	if cfg.Cloud != nil && cfg.Cloud.TLSCertificate != "" {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
				// always return same cert
				tlsCfg.certMu.Lock()
				defer tlsCfg.certMu.Unlock()
				return tlsCfg.tlsCert, nil
			},
			GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
				// always return same cert
				tlsCfg.certMu.Lock()
				defer tlsCfg.certMu.Unlock()
				return tlsCfg.tlsCert, nil
			},
		}
	}
	tlsCfg.Config = tlsConfig
	return tlsCfg
}

// UpdateCert updates the TLS certificate to be returned.
func (t *TLSConfig) UpdateCert(cfg *config.Config) error {
	cert, err := tls.X509KeyPair([]byte(cfg.Cloud.TLSCertificate), []byte(cfg.Cloud.TLSPrivateKey))
	if err != nil {
		return err
	}
	t.certMu.Lock()
	t.tlsCert = &cert
	t.certMu.Unlock()
	return nil
}

// ProcessConfig processes robot configs.
func ProcessConfig(in *config.Config, tlsCfg *TLSConfig) (*config.Config, error) {
	out := *in
	var selfCreds *rpc.Credentials
	if in.Cloud != nil {
		if in.Cloud.TLSCertificate != "" {
			if err := tlsCfg.UpdateCert(in); err != nil {
				return nil, err
			}
		}

		selfCreds = &rpc.Credentials{rutils.CredentialsTypeRobotSecret, in.Cloud.Secret}
		out.Network.TLSConfig = tlsCfg.Config // override
	}

	out.Remotes = make([]config.Remote, len(in.Remotes))
	copy(out.Remotes, in.Remotes)
	for idx, remote := range out.Remotes {
		remoteCopy := remote
		if in.Cloud == nil {
			remoteCopy.Auth.SignalingCreds = remoteCopy.Auth.Credentials
		} else {
			if remote.ManagedBy != in.Cloud.ManagedBy {
				continue
			}
			remoteCopy.Auth.Managed = true
			remoteCopy.Auth.SignalingServerAddress = in.Cloud.SignalingAddress
			remoteCopy.Auth.SignalingAuthEntity = in.Cloud.ID
			remoteCopy.Auth.SignalingCreds = selfCreds
		}
		out.Remotes[idx] = remoteCopy
	}
	return &out, nil
}

// RobotFromConfigPath is a helper to read and process a config given its path and then create a robot based on it.
func RobotFromConfigPath(ctx context.Context, cfgPath string, logger golog.Logger) (robot.LocalRobot, error) {
	cfg, err := config.Read(ctx, cfgPath, logger)
	if err != nil {
		logger.Fatal("cannot read config")
		return nil, err
	}
	return RobotFromConfig(ctx, cfg, logger)
}

// RobotFromConfig is a helper to process a config and then create a robot based on it.
func RobotFromConfig(ctx context.Context, cfg *config.Config, logger golog.Logger) (robot.LocalRobot, error) {
	tlsConfig := NewTLSConfig(cfg)
	processedCfg, err := ProcessConfig(cfg, tlsConfig)
	if err != nil {
		return nil, err
	}
	return robotimpl.New(ctx, processedCfg, logger)
}

// RunWeb starts the web server on the web service with web options and blocks until we close it.
func RunWeb(ctx context.Context, r robot.Robot, o web.Options, logger golog.Logger) (err error) {
	defer func() {
		if err != nil {
			err = utils.FilterOutError(err, context.Canceled)
			if err != nil {
				logger.Errorw("error running web", "error", err)
			}
		}
		err = multierr.Combine(err, utils.TryClose(ctx, r))
	}()
	svc, err := web.FromRobot(r)
	if err != nil {
		return err
	}
	if err := svc.Start(ctx, o); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

// RunWebWithConfig starts the web server on the web service with a robot config and blocks until we close it.
func RunWebWithConfig(ctx context.Context, r robot.Robot, cfg *config.Config, logger golog.Logger) error {
	o, err := web.OptionsFromConfig(cfg)
	if err != nil {
		return err
	}
	return RunWeb(ctx, r, o, logger)
}
