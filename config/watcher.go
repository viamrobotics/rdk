package config

import (
	"time"

	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/fsnotify/fsnotify"
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
func NewWatcher(config *Config, logger golog.Logger) (Watcher, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config.Cloud != nil {
		return newCloudWatcher(config.Cloud, logger), nil
	}
	if config.ConfigFilePath != "" {
		return newFSWatcher(config.ConfigFilePath, logger)
	}
	return noopWatcher{}, nil
}

// A cloudWatcher periodically fetches new configs from the cloud.
type cloudWatcher struct {
	configCh      chan *Config
	watcherDoneCh chan struct{}
	killCh        chan struct{}
}

// newCloudWatcher returns a cloudWatcher that will periodically fetch
// new configs from the cloud.
func newCloudWatcher(config *Cloud, logger golog.Logger) *cloudWatcher {
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
				newConfig, err := ReadFromCloud(config)
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
	return &cloudWatcher{
		configCh:      configCh,
		watcherDoneCh: watcherDoneCh,
		killCh:        killCh,
	}
}

func (w *cloudWatcher) Config() <-chan *Config {
	return w.configCh
}

func (w *cloudWatcher) Close() error {
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

// newFSWatcher returns a new v that will fetch new configs
// as soon as the underlying file is written to.
func newFSWatcher(configPath string, logger golog.Logger) (*fsConfigWatcher, error) {
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
					newConfig, err := Read(configPath)
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

// A noopWatcher does nothing.
type noopWatcher struct {
}

func (w noopWatcher) Config() <-chan *Config {
	return nil
}

func (w noopWatcher) Close() error {
	return nil
}
