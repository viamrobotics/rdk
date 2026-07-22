package config

import (
	"bytes"
	"context"
	"os"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
)

// The debug LogLevel value for a module config.
const moduleLogLevelDebug = "debug"

// A Watcher is responsible for watching for changes
// to a config from some source and delivering those changes
// to some destination.
type Watcher interface {
	Config() <-chan *Config
	Close() error
}

// NewWatcher returns an optimally selected Watcher based on the
// given config.
func NewWatcher(ctx context.Context, config *Config, logger logging.Logger, conn rpc.ClientConn) (Watcher, error) {
	if err := config.Ensure(false, logger); err != nil {
		return nil, err
	}
	if config.Cloud != nil {
		return newCloudWatcher(ctx, config, logger, conn), nil
	}
	if config.ConfigFilePath != "" {
		return newFSWatcher(ctx, config.ConfigFilePath, logger, conn)
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

// failuresBeforeWarning is how many consecutive failed cloud reads the watcher tolerates quietly
// before it starts warning, and how often it warns after that. At the default 10s refresh interval
// that is roughly once a minute. A single failed read is a blip; a run of them means the machine
// has silently stopped receiving config changes and someone should hear about it.
const failuresBeforeWarning = 6

// newCloudWatcher returns a cloudWatcher that will periodically fetch
// new configs from the cloud.
func newCloudWatcher(ctx context.Context, config *Config, logger logging.Logger, conn rpc.ClientConn) *cloudWatcher {
	configCh := make(chan *Config)
	watcherDoneCh := make(chan struct{})
	cancelCtx, cancel := context.WithCancel(ctx)

	nextCheckForNewCert := time.Now().Add(checkForNewCertInterval)
	machineID := config.Cloud.ID
	refreshInterval := config.Cloud.RefreshInterval
	// prevCloudConfig carries the cloud section (notably the TLS cert) from the previous successful
	// read forward in memory. It deliberately does not round-trip through the on-disk cache, which
	// is only written after reconfiguration completes and can lag many polls behind (RSDK-11851).
	//
	// Seed it with a copy for the same reason the loop below copies: the caller's Cloud is shared
	// with the config the robot is running on, and this must stay a private snapshot.
	prevCloudConfig := config.Cloud.Copy()
	utils.ManagedGo(func() {
		firstRead := true
		consecutiveFailures := 0
		for {
			// have first read with the watcher happen much faster in case the request timed out on the initial read on server startup
			interval := refreshInterval
			if firstRead {
				interval /= 5
				firstRead = false
			}
			if !utils.SelectContextOrWait(cancelCtx, interval) {
				return
			}
			var checkForNewCert bool
			if time.Now().After(nextCheckForNewCert) {
				checkForNewCert = true
			}
			newConfig, err := readFromCloud(cancelCtx, machineID, prevCloudConfig, checkForNewCert, logger, conn)
			if err != nil {
				consecutiveFailures++
				// A malformed config is a legitimate error. The robot keeps running its current config,
				// but we surface it loudly. A one-off failure to reach the cloud stays at debug since
				// the watcher retries -- but a run of them is no longer transient, so escalate.
				logFunc := logger.Debugw
				switch {
				case IsMalformedConfigError(err):
					logFunc = logger.Errorw
				case consecutiveFailures%failuresBeforeWarning == 0:
					logFunc = logger.Warnw
				}
				logFunc("could not apply new cloud config; keeping the current config",
					"error", err, "consecutive_failures", consecutiveFailures)
				continue
			}
			consecutiveFailures = 0
			// Carry the new cloud section forward as the next iteration's fallback. Copy rather
			// than alias newConfig.Cloud, since the robot may mutate the config we hand it. The
			// copy must not be allowed to fail: falling back to the older cloud section would hand
			// the robot an older TLS cert on the next poll, which is the reconfigure loop above.
			prevCloudConfig = newConfig.Cloud.Copy()
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
func newFSWatcher(ctx context.Context, configPath string, logger logging.Logger, conn rpc.ClientConn) (*fsConfigWatcher, error) {
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
				// Monitor WRITE and REMOVE events.
				// Editors that save in place WRITE over the monitored file.
				// Editors that save atomically write to a temp file and swap. Events on original file are: RENAME->CHMOD->REMOVE
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Remove == fsnotify.Remove {
					debounced(func() {
						logger.Info("On-disk config file changed. Reloading the config file.")
						//nolint:gosec
						rd, err := os.ReadFile(configPath)

						// Re-add to watcher. Will be a new inode if it was saved atomically.
						// Adding the same path twice (WRITE case) is a no-op (no error).
						// Old watches are auto removed from fsWatcher when file is deleted or renamed (REMOVE case).
						defer utils.UncheckedErrorFunc(func() error { return fsWatcher.Add(configPath) })

						if err != nil {
							logger.Errorw("error reading config file after write", "error", err)
							return
						}
						if bytes.Equal(rd, lastRd) {
							return
						}
						lastRd = rd
						newConfig, err := FromReader(cancelCtx, configPath, bytes.NewReader(rd), logger, conn)
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
