package config

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/fsnotify/fsnotify"
	"go.viam.com/utils"
)

// A Watcher is responsible for watching for changes
// to a config from some source and delivering those changes
// to some destination.
type Watcher interface {
	Config() <-chan *Config
}

// NewWatcher returns an optimally selected Watcher based on the
// given config.
func NewWatcher(ctx context.Context, config *Config, logger golog.Logger) (Watcher, error) {
	if err := config.Ensure(false); err != nil {
		return nil, err
	}
	if config.Cloud != nil {
		return newCloudWatcher(ctx, config.Cloud, logger), nil
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
func newCloudWatcher(ctx context.Context, config *Cloud, logger golog.Logger) *cloudWatcher {
	configCh := make(chan *Config)
	watcherDoneCh := make(chan struct{})
	cancelCtx, cancel := context.WithCancel(ctx)

	nextCheckForNewCert := time.Now().Add(checkForNewCertInterval)

	// TODO(https://github.com/viamrobotics/rdk/issues/45): in the future when the web app
	// supports gRPC streams, use that instead for pushed config updates;
	// for now just do a small interval.
	ticker := time.NewTicker(config.RefreshInterval)
	utils.ManagedGo(func() {
		for {
			if !utils.SelectContextOrWait(cancelCtx, config.RefreshInterval) {
				return
			}
			var checkForNewCert bool
			if time.Now().After(nextCheckForNewCert) {
				checkForNewCert = true
			}
			newConfig, _, err := ReadFromCloud(cancelCtx, config, false, checkForNewCert, logger)
			if err != nil {
				logger.Errorw("error reading cloud config", "error", err)
				continue
			}
			if checkForNewCert {
				nextCheckForNewCert = time.Now().Add(checkForNewCertInterval)
			}
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

func (w *cloudWatcher) Close() {
	w.cancel()
	<-w.watcherDoneCh
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
	utils.ManagedGo(func() {
		for {
			if cancelCtx.Err() != nil {
				return
			}
			select {
			case <-cancelCtx.Done():
				return
			case event := <-fsWatcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					newConfig, err := Read(cancelCtx, configPath, logger)
					if err != nil {
						logger.Errorw("error reading config after write", "error", err)
						continue
					}
					select {
					case <-cancelCtx.Done():
						return
					case configCh <- newConfig:
					}
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
