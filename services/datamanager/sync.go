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

	queueWaitTime time.Duration
	uploader      uploader
	cancelCtx     context.Context
	cancelFunc    func()
}

type uploader struct {
	// TODO: use a thread safe map or a lock
	inProgress map[string]struct{}
	uploadFn   func(path string) error
}

// newSyncer returns a new syncer.
func newSyncer(queuePath string, logger golog.Logger, captureDir string) syncer {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	ret := syncer{
		syncQueue:     queuePath,
		logger:        logger,
		captureDir:    captureDir,
		queueWaitTime: time.Minute,
		uploader: uploader{
			inProgress: map[string]struct{}{},
			// TODO: implement an uploadFn for uploading to cloud sync backend
			uploadFn: func(path string) error {
				return nil
			},
		},
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	return ret
}

// Enqueue moves files that are no longer being written to from captureDir to SyncQueue.
func (s *syncer) Enqueue(syncInterval time.Duration) {
	utils.PanicCapturingGo(func() {
		if err := os.MkdirAll(s.syncQueue, 0o700); err != nil {
			s.logger.Errorf("failed to make sync Enqueue: %v", err)
			return
		}
		ticker := time.NewTicker(syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-s.cancelCtx.Done():
				err := filepath.WalkDir(s.captureDir, s.queueFile)
				if err != nil {
					s.logger.Errorf("failed to move files to sync Enqueue: %v", err)
				}
				return
			case <-ticker.C:
				err := filepath.WalkDir(s.captureDir, s.queueFile)
				if err != nil {
					s.logger.Errorf("failed to move files to sync Enqueue: %v", err)
				}
			}
		}
	})
}

// UploadSynced syncs data to the backing storage system.
func (s *syncer) UploadSynced() {
	utils.PanicCapturingGo(func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.cancelCtx.Done():
				return
			case <-ticker.C:
				err := filepath.WalkDir(s.syncQueue, s.uploader.upload)
				if err != nil {
					s.logger.Errorf("failed to upload queued file: %v", err)
				}
			}
		}
	})
}

func (u *uploader) upload(path string, di fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if di.IsDir() {
		return nil
	}
	if _, ok := u.inProgress[path]; ok {
		return nil
	}

	// Mark upload as in progress.
	u.inProgress[path] = struct{}{}
	err = u.uploadFn(path)
	if err != nil {
		return err
	}

	// If upload completed successfully, unmark in-progress and delete file.
	// TODO: uncomment when sync is actually implemented. Until then, we don't want to delete data.
	// delete(u.inProgress, path)
	// err = os.Remove(path)
	// if err != nil {
	//  	return err
	// }
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

	// If it's been written to in the last s.queueWaitTime, it's still active and shouldn't be queued.
	if time.Since(fileInfo.ModTime()) < s.queueWaitTime {
		return nil
	}

	subPath, err := s.getPathUnderCaptureDir(filePath)
	if err != nil {
		return errors.Errorf("could not get path under capture directory: %v", err)
	}

	if err = os.MkdirAll(filepath.Dir(path.Join(s.syncQueue, subPath)), 0o700); err != nil {
		return errors.Errorf("failed create directories under sync Enqueue: %v", err)
	}

	err = os.Rename(filePath, path.Join(s.syncQueue, subPath))
	if err != nil {
		return errors.Errorf("failed to move file to sync Enqueue: %v", err)
	}
	return nil
}

func (s *syncer) getPathUnderCaptureDir(filePath string) (string, error) {
	if idx := strings.Index(filePath, s.captureDir); idx != -1 {
		return filePath[idx+len(s.captureDir):], nil
	}
	return "", errors.Errorf("file path %s is not under capture directory %s", filePath, s.captureDir)
}

// Close closes all resources (goroutines) associated with s.
func (s *syncer) Close() {
	s.cancelFunc()
}
