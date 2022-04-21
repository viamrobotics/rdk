package datamanager

import (
	"context"
	"fmt"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type SyncManager interface {
	Queue(syncIntervalMins int) error
	Upload()
	Close() error
	SetCaptureDir(dir string)
}

type SyncManagerImpl struct {
	captureDir string
	syncQueue  string
	logger     golog.Logger

	cancelCtx  context.Context
	cancelFunc func()
}

// New returns a new data manager service for the given robot.
func NewSyncManager(ctx context.Context, queuePath string, logger golog.Logger, captureDir string) SyncManager {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	ret := &SyncManagerImpl{
		syncQueue:  queuePath,
		logger:     logger,
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
			err := filepath.WalkDir(s.captureDir, s.queue)
			if err != nil {
				s.logger.Errorf("failed to move files to sync queue: %v", err)
			}
			return nil
		case <-ticker.C:
			s.logger.Info(s.captureDir)
			err := filepath.WalkDir(s.captureDir, s.queue)
			if err != nil {
				s.logger.Errorf("failed to move files to sync queue: %v", err)
			}
		}
	}
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

func (s *SyncManagerImpl) SetCaptureDir(dir string) {
	s.captureDir = dir
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

// TODO: implement
func (s *SyncManagerImpl) queue(filePath string, di fs.DirEntry, err error) error {
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

	// TODO: sufficient?
	// If it's been written to in the last minute, it's still active and shouldn't be queued.
	if time.Now().Sub(fileInfo.ModTime()) < time.Minute {
		fmt.Println(filePath + " " + string(time.Now().Sub(fileInfo.ModTime())))
		return nil
	}

	subPath, err := s.getPathUnderCaptureDir(filePath)
	if err != nil {
		return errors.Errorf("could not get path under capture directory: %v", err)
	}

	if err = os.MkdirAll(filepath.Dir(path.Join(SyncQueue, subPath)), 0o700); err != nil {
		return errors.Errorf("failed create directories under sync queue: %v", err)
	}

	// TODO: create all necessary directories under sync queue before moving
	err = os.Rename(filePath, path.Join(SyncQueue, subPath))
	if err != nil {
		return errors.Errorf("failed to move file to sync queue: %v", err)
	}
	return nil
}

func (s *SyncManagerImpl) getPathUnderCaptureDir(filePath string) (string, error) {
	if idx := strings.Index(filePath, s.captureDir); idx != -1 {
		return filePath[idx+len(s.captureDir):], nil
	} else {
		return "", errors.Errorf("file path %s is not under capture directory %s", filePath, s.captureDir)
	}
}

//func getDataSyncDir(subtypeName string, componentName string) string {
//	return filepath.Join(SyncQueue, subtypeName, componentName)
//}
//
//// Create the data sync queue subdirectory containing a given component's data.
//func createDataSyncDir(subtypeName string, componentName string) error {
//	fileDir := getDataSyncDir(subtypeName, componentName)
//	if err := os.MkdirAll(fileDir, 0o700); err != nil {
//		return err
//	}
//	return nil
//}

func (s *SyncManagerImpl) Close() error {
	s.cancelFunc()
	return nil
}
