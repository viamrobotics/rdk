package datamanager

import (
	"context"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"
)

type SyncManager interface {
	Queue(syncIntervalMins int) error
	Upload()
	Close() error
}

type SyncManagerImpl struct {
	syncQueue  string
	collectors *map[componentMethodMetadata]collectorParams
	logger     golog.Logger
	// TODO: don't think the Sync Manager should be aware of this... think of way to extract
	captureDir string

	cancelCtx  context.Context
	cancelFunc func()
}

// New returns a new data manager service for the given robot.
func NewSyncManager(ctx context.Context, queuePath string, collectors *map[componentMethodMetadata]collectorParams,
	logger golog.Logger, captureDir string) SyncManager {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	ret := &SyncManagerImpl{
		syncQueue:  queuePath,
		logger:     logger,
		collectors: collectors,
		captureDir: captureDir,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	return ret
}

// Sync syncs data to the backing storage system.
func (s *SyncManagerImpl) Queue(syncIntervalMins int) error {
	if err := os.MkdirAll(SyncQueue, 0o700); err != nil {
		return errors.Errorf("failed to make sync queue: %v", err)
	}
	ticker := time.NewTicker(time.Minute * time.Duration(syncIntervalMins))
	defer ticker.Stop()

	for {
		select {
		case <-s.cancelCtx.Done():
			err := s.moveToSyncQueue(false)
			if err != nil {
				return errors.Errorf("failed to move files to sync queue: %v", err)
			}
			return nil
		case <-ticker.C:
			err := s.moveToSyncQueue(true)
			if err != nil {
				return errors.Errorf("failed to move files to sync queue: %v", err)
			}
		}
	}
}

func (s *SyncManagerImpl) moveToSyncQueue(createNewTargets bool) error {
	for component, collector := range *s.collectors {
		oldTarget := collector.Collector.GetTarget()
		// Create new target and set it.
		if createNewTargets {
			nextTarget, err := createDataCaptureFile(s.captureDir, collector.Attributes.Type, component.ComponentName)
			if err != nil {
				return errors.Errorf("failed to create new data capture file: %v", err)
			}
			collector.Collector.SetTarget(nextTarget)
		} else {
			collector.Collector.Close()
		}

		// Move collector file to SYNC_QUEUE
		err := oldTarget.Close()
		if err != nil {
			return errors.Errorf("failed to close old data capture file: %v", err)
		}
		err = os.Rename(oldTarget.Name(),
			path.Join(
				getDataSyncDir(collector.Attributes.Type, component.ComponentName),
				filepath.Base(oldTarget.Name())))
		if err != nil {
			return errors.Errorf("failed to move file to sync queue: %v", err)
		}
	}
	return nil
}

// Upload syncs data to the backing storage system.
func (s *SyncManagerImpl) Upload() {
	for {
		select {
		case <-s.cancelCtx.Done():
			return
		default:
			err := filepath.WalkDir(SyncQueue, s.uploadQueuedFile)
			if err != nil {
				s.logger.Errorf("failed to upload queued file: %v", err)
			}
		}
	}
}

// TODO: implement
func (s *SyncManagerImpl) uploadQueuedFile(path string, di fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if di.IsDir() {
		return nil
	}
	//s.logger.Debugf("Visited: %s\n", path)
	return nil
}

func getDataSyncDir(subtypeName string, componentName string) string {
	return filepath.Join(SyncQueue, subtypeName, componentName)
}

// Create the data sync queue subdirectory containing a given component's data.
func createDataSyncDir(subtypeName string, componentName string) error {
	fileDir := getDataSyncDir(subtypeName, componentName)
	if err := os.MkdirAll(fileDir, 0o700); err != nil {
		return err
	}
	return nil
}

func (s *SyncManagerImpl) Close() error {
	s.cancelFunc()
	return nil
}
