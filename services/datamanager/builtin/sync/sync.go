// Package sync implements datasync for the builtin datamanger
package sync

import (
	"context"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/utils"
)

var (
	// DefaultDeleteEveryNth temporarily public for tests.
	DefaultDeleteEveryNth = 5
	// DeletionTicker temporarily public for tests.
	DeletionTicker = clk.New()
	// FilesystemPollInterval temporarily public for tests.
	FilesystemPollInterval = 30 * time.Second
	grpcConnectionTimeout  = 10 * time.Second
)

const (
	// Default time to wait in milliseconds to check if a file has been modified.
	defaultFileLastModifiedMillis = 10000.0
	// FailedDir is a subdirectory of the capture directory that holds any files that could not be synced.
	FailedDir = "failed"
	// DefaultMaxParallelSyncRoutines is the maximum number of sync goroutines that can be running at once.
	DefaultMaxParallelSyncRoutines = 100
)

type selectiveSyncer interface {
	sensor.Sensor
}

// Config is the sync config from builtin.
type Config struct {
	AdditionalSyncPaths        []string
	CaptureDir                 string
	CaptureDisabled            bool
	DeleteEveryNthWhenDiskFull int
	FileLastModifiedMillis     int
	MaximumNumSyncThreads      int
	SyncDisabled               bool
	SelectiveSyncerName        string
	SyncIntervalMins           float64
	Tags                       []string
}

type configWithDeps struct {
	config Config

	cloudConnSvc cloud.ConnectionService
	syncSensor   sensor.Sensor
}

func (c configWithDeps) disabled() bool {
	return c.config.SyncDisabled || utils.Float64AlmostEqual(c.config.SyncIntervalMins, 0.0, 0.00001)
}

func (c Config) Equal(o Config) bool {
	return reflect.DeepEqual(c.AdditionalSyncPaths, o.AdditionalSyncPaths) &&
		c.CaptureDir == o.CaptureDir &&
		c.CaptureDisabled == o.CaptureDisabled &&
		c.DeleteEveryNthWhenDiskFull == o.DeleteEveryNthWhenDiskFull &&
		c.FileLastModifiedMillis == o.FileLastModifiedMillis &&
		c.MaximumNumSyncThreads == o.MaximumNumSyncThreads &&
		c.SyncDisabled == o.SyncDisabled &&
		c.SelectiveSyncerName == o.SelectiveSyncerName &&
		c.SyncIntervalMins == o.SyncIntervalMins &&
		reflect.DeepEqual(c.Tags, o.Tags)
}

func (c configWithDeps) Equal(o configWithDeps) bool {
	return c.config.Equal(o.config) &&
		c.cloudConnSvc == o.cloudConnSvc &&
		c.syncSensor == o.syncSensor
}

func (c Config) applyDefaults(cloudConnSvc cloud.ConnectionService, syncSensor sensor.Sensor) configWithDeps {
	newMaxSyncThreadValue := DefaultMaxParallelSyncRoutines
	if c.MaximumNumSyncThreads != 0 {
		newMaxSyncThreadValue = c.MaximumNumSyncThreads
	}
	c.MaximumNumSyncThreads = newMaxSyncThreadValue

	deleteEveryNthValue := DefaultDeleteEveryNth
	if c.DeleteEveryNthWhenDiskFull != 0 {
		deleteEveryNthValue = c.DeleteEveryNthWhenDiskFull
	}
	c.DeleteEveryNthWhenDiskFull = deleteEveryNthValue

	fileLastModifiedMillis := c.FileLastModifiedMillis
	if fileLastModifiedMillis <= 0 {
		fileLastModifiedMillis = defaultFileLastModifiedMillis
	}
	c.FileLastModifiedMillis = fileLastModifiedMillis

	return configWithDeps{
		config:       c,
		cloudConnSvc: cloudConnSvc,
		syncSensor:   syncSensor,
	}
}

// TODO: Confirm this works for an empty config.
func (c Config) syncPaths() []string {
	return append([]string{c.CaptureDir}, c.AdditionalSyncPaths...)
}

// Sync manages uploading metrics from files to the cloud & deleting the upload files.
// There must be only one Capture per DataManager. The lifecycle of a Capture is:
//
// - NewSync
// - Reconfigure (any number of times)
// - Close (once).
type Sync struct {
	logger              logging.Logger
	clock               clk.Clock
	flushCollectors     func()
	fileDeletingWorkers utils.StoppableWorkers

	applyConfigWorkerStarted atomic.Bool
	applyConfigWorkers       utils.StoppableWorkers
	// DataSyncServiceClientConstructor temporarily public for tests
	DataSyncServiceClientConstructor func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient

	// syncSchedulerWorkers utils.StoppableWorkers
	syncerMu sync.Mutex
	// Syncer temporarily public for tests
	Syncer *Syncer
	// cloudConn rpc.ClientConn

	appliedConfigWithDepsMu sync.Mutex
	appliedConfigWithDeps   configWithDeps

	proposedConfigWithDepsMu sync.Mutex
	proposedConfigWithDeps   configWithDeps
}

// New creates a new Manager.
func New(
	logger logging.Logger,
	clk clk.Clock,
	flushCollectors func(),
) *Sync {
	return &Sync{
		flushCollectors:                  flushCollectors,
		clock:                            clk,
		logger:                           logger,
		DataSyncServiceClientConstructor: v1.NewDataSyncServiceClient,
		applyConfigWorkers:               utils.NewStoppableWorkers(),
		fileDeletingWorkers:              utils.NewStoppableWorkers(),
	}
}

func syncSensorFromDeps(selectiveSyncerName string, deps resource.Dependencies, logger logging.Logger) sensor.Sensor {
	var syncSensor sensor.Sensor
	if selectiveSyncerName != "" {
		tmp, err := sensor.FromDependencies(deps, selectiveSyncerName)
		if err != nil {
			logger.Errorw(
				"unable to initialize selective syncer; will not sync at all until fixed or removed from config", "error", err.Error())
		}
		syncSensor = tmp
	}
	return syncSensor
}

func (s *Sync) proposedConfig() configWithDeps {
	s.proposedConfigWithDepsMu.Lock()
	defer s.proposedConfigWithDepsMu.Unlock()
	return s.proposedConfigWithDeps
}

func (s *Sync) appliedConfig() configWithDeps {
	s.appliedConfigWithDepsMu.Lock()
	defer s.appliedConfigWithDepsMu.Unlock()
	return s.appliedConfigWithDeps
}

func (s *Sync) setAppliedConfig(c configWithDeps) {
	s.appliedConfigWithDepsMu.Lock()
	defer s.appliedConfigWithDepsMu.Unlock()
	s.appliedConfigWithDeps = c
}

// ConfigApplied returns true when Sync has applied the proposed config
func (s *Sync) ConfigApplied() bool {
	return s.proposedConfig().Equal(s.appliedConfig())
}

// Reconfigure reconfigures Sync.
// It is only called by the builtin data manager
// https://github.com/dgottlieb/rdk/blob/72f5b567db2cb2ca08b9752b8710d1e4e784077c/services/datamanager/datasync/manager.go
// https://github.com/dgottlieb/rdk/blob/72f5b567db2cb2ca08b9752b8710d1e4e784077c/services/datamanager/builtin/builtin.go#L144
func (s *Sync) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	configWithoutDefaults Config,
	cloudConnSvc cloud.ConnectionService,
) {
	s.logger.Info("Reconfigure START")
	defer s.logger.Info("Reconfigure END")
	syncSensor := syncSensorFromDeps(configWithoutDefaults.SelectiveSyncerName, deps, s.logger)

	config := configWithoutDefaults.applyDefaults(cloudConnSvc, syncSensor)

	s.proposedConfigWithDepsMu.Lock()
	defer s.proposedConfigWithDepsMu.Unlock()
	if !s.proposedConfigWithDeps.Equal(config) {
		s.proposedConfigWithDeps = config
	}

	if !s.applyConfigWorkerStarted.Swap(true) {
		s.applyConfigWorkers = utils.NewStoppableWorkers(s.applyConfigLoop)
	}
}

// applyConfigLoop runs until Close() is called
// Immediately on first execution and every second afterwards it
// checks if the datasync configuration has changes which
// have not propagated to datasync.
// If so it propagates the changes and marks the datasync configuration as propagated.
// Otherwise it sleeps for another second.
// Takes the lock every iteration.
func (s *Sync) applyConfigLoop(ctx context.Context) {
	if err := s.applyConfig(ctx); err != nil {
		return
	}
	for goutils.SelectContextOrWait(ctx, time.Millisecond*100) {
		if err := s.applyConfig(ctx); err != nil {
			return
		}
	}
}

// applyConfig applies the data sync config set in the previous Reconfigure call.
func (s *Sync) applyConfig(ctx context.Context) error {
	proposed := s.proposedConfig()
	if proposed.Equal(s.appliedConfig()) {
		return nil
	}
	s.logger.Info("propagateDataSyncConfig START")
	defer s.logger.Info("propagateDataSyncConfig END")

	s.fileDeletingWorkers.Stop()
	if proposed.disabled() {
		s.logger.Debug("sync is disabled")
		s.closeSyncer()
		s.setAppliedConfig(proposed)
		return nil
	}

	s.logger.Debug("sync is enabled")
	syncer, err := s.maybeStartOrUpdateSyncer(ctx, proposed)
	if err != nil {
		return err
	}

	// if datacapture is enabled, kick off a go routine to handle disk space filling due to
	// cached datacapture files
	if !proposed.config.CaptureDisabled {
		s.fileDeletingWorkers = utils.NewStoppableWorkers(func(ctx context.Context) {
			deleteExcessFiles(
				ctx,
				proposed.config.CaptureDir,
				proposed.config.DeleteEveryNthWhenDiskFull,
				syncer,
				s.logger,
			)
		})
	}

	s.setAppliedConfig(proposed)
	return nil
}

func (s *Sync) maybeStartOrUpdateSyncer(
	ctx context.Context,
	configWithDeps configWithDeps,
) (*Syncer, error) {
	if configWithDeps.config.MaximumNumSyncThreads != s.appliedConfigWithDeps.config.MaximumNumSyncThreads {
		s.logger.Debug("closing syncer as number of threads has changed")
		s.closeSyncer()
	}

	syncer, err := s.initSyncer(ctx, configWithDeps, true)
	if err != nil {
		if errors.Is(err, cloud.ErrNotCloudManaged) {
			s.logger.Debug("Using no-op sync manager when not cloud managed")
			return nil, err
		}
		s.logger.Infof("initSyncer err: %s", err.Error())
		// NOTE: it is ok to return a nil syncer here as deleteExcessFiles can tolerate a nil syncer
		//nolint:nilnil
		return nil, nil
	}

	return syncer, nil
}

// readyToSync is a method for getting the bool reading from the selective sync sensor
// for determining whether the key is present and what its value is.
func readyToSync(ctx context.Context, s selectiveSyncer, logger logging.Logger) (readyToSync bool) {
	readyToSync = false
	readings, err := s.Readings(ctx, nil)
	if err != nil {
		logger.CErrorw(ctx, "error getting readings from selective syncer", "error", err.Error())
		return
	}
	readyToSyncVal, ok := readings[datamanager.ShouldSyncKey]
	if !ok {
		logger.CErrorf(ctx, "value for should sync key %s not present in readings", datamanager.ShouldSyncKey)
		return
	}
	readyToSyncBool, err := utils.AssertType[bool](readyToSyncVal)
	if err != nil {
		logger.CErrorw(ctx, "error converting should sync key to bool", "key", datamanager.ShouldSyncKey, "error", err.Error())
		return
	}
	readyToSync = readyToSyncBool
	return
}

func deleteExcessFiles(ctx context.Context, captureDir string, deleteEveryNth int, syncer *Syncer, logger logging.Logger,
) {
	if runtime.GOOS == "android" {
		logger.Debug("file deletion if disk is full is not currently supported on Android")
		return
	}
	t := DeletionTicker.Ticker(FilesystemPollInterval)
	defer t.Stop()
	for {
		if err := ctx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Errorw("data manager context closed unexpectedly in filesystem polling", "error", err)
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			logger.Debug("checking disk usage")
			shouldDelete, err := shouldDeleteBasedOnDiskUsage(ctx, captureDir, logger)
			if err != nil {
				logger.Warnw("error checking file system stats", "error", err)
			}
			if shouldDelete {
				start := time.Now()
				deletedFileCount, err := deleteFiles(ctx, syncer, deleteEveryNth, captureDir, logger)
				duration := time.Since(start)
				if err != nil {
					logger.Errorw("error deleting cached datacapture files", "error", err, "execution time", duration.Seconds())
				} else {
					logger.Infof("%v files have been deleted to avoid the disk filling up, execution time: %f", deletedFileCount, duration.Seconds())
				}
			}
		}
	}
}

// Close releases all resources managed by data_manager.
func (s *Sync) Close() {
	s.applyConfigWorkers.Stop()
	s.fileDeletingWorkers.Stop()
	s.closeSyncer()
}

func (s *Sync) closeSyncer() {
	s.syncerMu.Lock()
	defer s.syncerMu.Unlock()
	if s.Syncer != nil {
		// If previously we were syncing, close the old syncer and cancel the old updateCollectors goroutine.
		s.Syncer.Close()
		s.Syncer = nil
	}
}

// initSyncer returns the existing syncer if there is one
// otherwise it creates a new connection with app.viam.com and a new syncer
// if schedule is true it starts the syncer scheduler.
func (s *Sync) initSyncer(
	ctx context.Context,
	configWithDeps configWithDeps,
	schedule bool,
) (*Syncer, error) {
	s.syncerMu.Lock()
	if syncer := s.Syncer; syncer != nil {
		s.syncerMu.Unlock()
		return syncer, nil
	}
	s.syncerMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, grpcConnectionTimeout)
	defer cancel()
	partID, conn, err := configWithDeps.cloudConnSvc.AcquireConnection(ctx)
	if err != nil {
		return nil, err
	}

	s.syncerMu.Lock()
	defer s.syncerMu.Unlock()
	if syncer := s.Syncer; syncer != nil {
		conn.Close()
		return syncer, nil
	}

	syncer := NewSyncer(
		configWithDeps,
		partID,
		s.DataSyncServiceClientConstructor,
		conn,
		s.clock,
		s.flushCollectors,
		schedule,
		s.logger)
	s.Syncer = syncer
	return syncer, nil
}

// TODO: Determine desired behavior if sync is disabled. Do we wan to allow manual syncs, then?
//       If so, how could a user cancel it?

// Sync performs a non-scheduled sync of the data in the capture directory.
// If automated sync is also enabled, calling Sync will upload the files,
// regardless of whether or not is the scheduled time.
func (s *Sync) Sync(ctx context.Context, _ map[string]interface{}) error {
	s.proposedConfigWithDepsMu.Lock()
	config := s.proposedConfigWithDeps
	s.proposedConfigWithDepsMu.Unlock()

	syncer, err := s.initSyncer(ctx, config, false)
	if err != nil {
		return err
	}

	syncer.flushAndSendFilesToSync(ctx)
	return nil
}
