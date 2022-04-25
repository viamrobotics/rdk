package datamanager

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

// syncer is responsible for enqueuing files in captureDir and syncing them to the cloud.
type syncer struct {
	captureDir string
	syncQueue  string
	logger     golog.Logger

	cancelCtx  context.Context
	cancelFunc func()
}

// newSyncManager returns a new data manager service for the given robot.
func newSyncManager(queuePath string, logger golog.Logger, captureDir string) syncer {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	ret := syncer{
		syncQueue:  queuePath,
		logger:     logger,
		captureDir: captureDir,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	return ret
}

// enqueue moves files that are no longer being written to from captureDir to SyncQueue.
func (s *syncer) enqueue(syncIntervalMins int) {
	utils.PanicCapturingGo(func() {
		if err := os.MkdirAll(SyncQueue, 0o700); err != nil {
			s.logger.Errorf("failed to make sync enqueue: %v", err)
			return
		}
		ticker := time.NewTicker(time.Minute * time.Duration(syncIntervalMins))
		defer ticker.Stop()

		for {
			select {
			case <-s.cancelCtx.Done():
				err := filepath.WalkDir(s.captureDir, s.queueFile)
				if err != nil {
					s.logger.Errorf("failed to move files to sync enqueue: %v", err)
				}
				return
			case <-ticker.C:
				s.logger.Info(s.captureDir)
				err := filepath.WalkDir(s.captureDir, s.queueFile)
				if err != nil {
					s.logger.Errorf("failed to move files to sync enqueue: %v", err)
				}
			}
		}
	})
}

// upload syncs data to the backing storage system.
func (s *syncer) upload() {
	utils.PanicCapturingGo(func() {
		for {
			select {
			case <-s.cancelCtx.Done():
				return
			default:
				err := filepath.WalkDir(SyncQueue, s.uploadFile)
				if err != nil {
					s.logger.Errorf("failed to upload queued file: %v", err)
				}
			}
		}
	})
}

// TODO: implement.
func (s *syncer) uploadFile(path string, di fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if di.IsDir() {
		return nil
	}
	// s.logger.Debugf("Visited: %s\n", path)
	return nil
}

func (s *syncer) queueFile(filePath string, di fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if di.IsDir() {
		return nil
	}

	fileInfo, err := di.Info()
	if err != nil {
		return errors.Errorf("failed to get file info for filepath %s: %v", filePath, err)
	}

	// If it's been written to in the last minute, it's still active and shouldn't be queued.
	if time.Since(fileInfo.ModTime()) < time.Minute {
		return nil
	}

	subPath, err := s.getPathUnderCaptureDir(filePath)
	if err != nil {
		return errors.Errorf("could not get path under capture directory: %v", err)
	}

	if err = os.MkdirAll(filepath.Dir(path.Join(SyncQueue, subPath)), 0o700); err != nil {
		return errors.Errorf("failed create directories under sync enqueue: %v", err)
	}

	// TODO: create all necessary directories under sync enqueue before moving
	err = os.Rename(filePath, path.Join(SyncQueue, subPath))
	if err != nil {
		return errors.Errorf("failed to move file to sync enqueue: %v", err)
	}
	return nil
}

func (s *syncer) getPathUnderCaptureDir(filePath string) (string, error) {
	if idx := strings.Index(filePath, s.captureDir); idx != -1 {
		return filePath[idx+len(s.captureDir):], nil
	}
	return "", errors.Errorf("file path %s is not under capture directory %s", filePath, s.captureDir)
}

// close closes all resources (goroutines) associated with s.
func (s *syncer) close() {
	s.cancelFunc()
}
