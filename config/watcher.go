package config

import (
	"bytes"
	"context"
	"os"
	"time"

	"github.com/bep/debounce"
	"github.com/edaniels/golog"
	"github.com/fsnotify/fsnotify"
	"go.viam.com/utils"
)

// A Watcher is responsible for watching for changes
// to a config from some source and delivering those changes
// to some destination.
type Watcher interface {
	Config() <-chan *Config
	Close() error
}

// NewWatcher returns an optimally selected Watcher based on the
// given config.
func NewWatcher(ctx context.Context, config *Config, logger golog.Logger) (Watcher, error) {
	if err := config.Ensure(false, logger); err != nil {
		return nil, err
	}
	if config.Cloud != nil {
		return newCloudWatcher(ctx, config, logger), nil
	}
	if config.ConfigFilePath != "" {
		return newFSWatcher(ctx, config.ConfigFilePath, logger)
	}
	return noopWatcher{}, nil
}

// A cloudWatcher periodically fetches new configs from the cloud.
type cloudWatcher struct {
	configCh      chan *Config
	watcherDoneCh chan struct{}
	cancel        func()
}

const checkForNewCertInterval = time.Hour

// newCloudWatcher returns a cloudWatcher that will periodically fetch
// new configs from the cloud.
func newCloudWatcher(ctx context.Context, config *Config, logger golog.Logger) *cloudWatcher {
	configCh := make(chan *Config)
	watcherDoneCh := make(chan struct{})
	cancelCtx, cancel := context.WithCancel(ctx)

	nextCheckForNewCert := time.Now().Add(checkForNewCertInterval)

	ticker := time.NewTicker(config.Cloud.RefreshInterval)

	var prevCfg *Config
	utils.ManagedGo(func() {
		for {
			if !utils.SelectContextOrWait(cancelCtx, config.Cloud.RefreshInterval) {
				return
			}
			var checkForNewCert bool
			if time.Now().After(nextCheckForNewCert) {
				checkForNewCert = true
			}
			newConfig, err := readFromCloud(cancelCtx, config, prevCfg, false, checkForNewCert, logger)
			if err != nil {
				logger.Errorw("error reading cloud config", "error", err)
				continue
			}
			if cp, err := newConfig.CopyOnlyPublicFields(); err == nil {
				prevCfg = cp
			}
			if checkForNewCert {
				nextCheckForNewCert = time.Now().Add(checkForNewCertInterval)
			}
			UpdateCloudConfigDebug(newConfig.Debug)
			select {
			case <-cancelCtx.Done():
				return
			case configCh <- newConfig:
			}
		}
	}, func() {
		ticker.Stop()
		close(watcherDoneCh)
	})
	return &cloudWatcher{
		configCh:      configCh,
		watcherDoneCh: watcherDoneCh,
		cancel:        cancel,
	}
}

func (w *cloudWatcher) Config() <-chan *Config {
	return w.configCh
}

func (w *cloudWatcher) Close() error {
	w.cancel()
	<-w.watcherDoneCh
	return nil
}

// A fsConfigWatcher fetches new configs from an underlying file when written to.
type fsConfigWatcher struct {
	fsWatcher     *fsnotify.Watcher
	configCh      chan *Config
	watcherDoneCh chan struct{}
	cancel        func()
}

// newFSWatcher returns a new v that will fetch new configs
// as soon as the underlying file is written to.
func newFSWatcher(ctx context.Context, configPath string, logger golog.Logger) (*fsConfigWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fsWatcher.Add(configPath); err != nil {
		return nil, err
	}
	configCh := make(chan *Config)
	watcherDoneCh := make(chan struct{})
	cancelCtx, cancel := context.WithCancel(ctx)
	var lastRd []byte
	utils.ManagedGo(func() {
		debounced := debounce.New(time.Millisecond * 500)
		for {
			if cancelCtx.Err() != nil {
				return
			}
			select {
			case <-cancelCtx.Done():
				return
			case event := <-fsWatcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					debounced(func() {
						//nolint:gosec
						rd, err := os.ReadFile(configPath)
						if err != nil {
							logger.Errorw("error reading config file after write", "error", err)
							return
						}
						if bytes.Equal(rd, lastRd) {
							return
						}
						lastRd = rd
						newConfig, err := FromReader(cancelCtx, configPath, bytes.NewReader(rd), logger)
						if err != nil {
							logger.Errorw("error reading config after write", "error", err)
							return
						}
						UpdateFileConfigDebug(newConfig.Debug)
						select {
						case <-cancelCtx.Done():
							return
						case configCh <- newConfig:
						}
					})
				}
			}
		}
	}, func() {
		close(watcherDoneCh)
	})
	return &fsConfigWatcher{
		fsWatcher:     fsWatcher,
		configCh:      configCh,
		watcherDoneCh: watcherDoneCh,
		cancel:        cancel,
	}, nil
}

func (w *fsConfigWatcher) Config() <-chan *Config {
	return w.configCh
}

func (w *fsConfigWatcher) Close() error {
	w.cancel()
	<-w.watcherDoneCh
	return w.fsWatcher.Close()
}

// A noopWatcher does nothing.
type noopWatcher struct{}

func (w noopWatcher) Config() <-chan *Config {
	return nil
}

func (w noopWatcher) Close() error {
	return nil
}
