package api

import (
	"time"

	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
	"github.com/fsnotify/fsnotify"
)

// A ConfigWatcher is responsible for watching for changes
// to a config from some source and delivering those changes
// to some destination.
type ConfigWatcher interface {
	Config() <-chan *Config
	Close() error
}

// NewConfigWatcher returns an optimally selected ConfigWatcher based on the
// given config.
func NewConfigWatcher(config *Config, logger golog.Logger) (ConfigWatcher, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config.Cloud != nil {
		return newCloudConfigWatcher(config.Cloud, logger), nil
	}
	if config.ConfigFilePath != "" {
		return newFSConfigWatcher(config.ConfigFilePath, logger)
	}
	return noopConfigWatcher{}, nil
}

// A cloudConfigWatcher periodically fetches new configs from the cloud.
type cloudConfigWatcher struct {
	configCh      chan *Config
	watcherDoneCh chan struct{}
	killCh        chan struct{}
}

// newCloudConfigWatcher returns a cloudConfigWatcher that will periodically fetch
// new configs from the cloud.
func newCloudConfigWatcher(config *CloudConfig, logger golog.Logger) *cloudConfigWatcher {
	configCh := make(chan *Config)
	watcherDoneCh := make(chan struct{})
	killCh := make(chan struct{})

	// TODO(https://github.com/viamrobotics/robotcore/issues/45): in the future when the web app
	// supports gRPC streams, use that instead for pushed config updates;
	// for now just do a small interval.
	ticker := time.NewTicker(config.RefreshInterval)
	utils.ManagedGo(func() {
		for {
			select {
			case <-killCh:
				return
			case <-ticker.C:
				newConfig, err := ReadConfigFromCloud(config)
				if err != nil {
					logger.Errorw("error reading cloud config", "error", err)
					continue
				}
				select {
				case <-killCh:
					return
				case configCh <- newConfig:
				}
			}
		}
	}, func() {
		ticker.Stop()
		close(watcherDoneCh)
	})
	return &cloudConfigWatcher{
		configCh:      configCh,
		watcherDoneCh: watcherDoneCh,
		killCh:        killCh,
	}
}

func (w *cloudConfigWatcher) Config() <-chan *Config {
	return w.configCh
}

func (w *cloudConfigWatcher) Close() error {
	close(w.killCh)
	<-w.watcherDoneCh
	return nil
}

// A fsConfigWatcher fetches new configs from an underlying file when written to.
type fsConfigWatcher struct {
	fsWatcher     *fsnotify.Watcher
	configCh      chan *Config
	watcherDoneCh chan struct{}
	killCh        chan struct{}
}

// newFSConfigWatcher returns a new fsConfigWatcher that will fetch new configs
// as soon as the underlying file is written to.
func newFSConfigWatcher(configPath string, logger golog.Logger) (*fsConfigWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fsWatcher.Add(configPath); err != nil {
		return nil, err
	}
	configCh := make(chan *Config)
	watcherDoneCh := make(chan struct{})
	killCh := make(chan struct{})
	utils.ManagedGo(func() {
		for {
			select {
			case <-killCh:
				return
			case event := <-fsWatcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					newConfig, err := ReadConfig(configPath)
					if err != nil {
						logger.Errorw("error reading config after write", "error", err)
						continue
					}
					select {
					case <-killCh:
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
		killCh:        killCh,
	}, nil
}

func (w *fsConfigWatcher) Config() <-chan *Config {
	return w.configCh
}

func (w *fsConfigWatcher) Close() error {
	close(w.killCh)
	<-w.watcherDoneCh
	return w.fsWatcher.Close()
}

// A noopConfigWatcher does nothing.
type noopConfigWatcher struct {
}

func (w noopConfigWatcher) Config() <-chan *Config {
	return nil
}

func (w noopConfigWatcher) Close() error {
	return nil
}
