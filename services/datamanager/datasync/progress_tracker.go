package datasync

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/pkg/errors"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

var viamProgressDotDir = filepath.Join(os.Getenv("HOME"), ".viam", "progress")

type progressTracker struct {
	lock        *sync.Mutex
	m           map[string]struct{}
	progressDir string
}

func (pt *progressTracker) inProgress(k string) bool {
	pt.lock.Lock()
	defer pt.lock.Unlock()
	_, ok := pt.m[k]
	return ok
}

func (pt *progressTracker) mark(k string) {
	pt.lock.Lock()
	pt.m[k] = struct{}{}
	pt.lock.Unlock()
}

func (pt *progressTracker) unmark(k string) {
	pt.lock.Lock()
	delete(pt.m, k)
	pt.lock.Unlock()
}

func (pt *progressTracker) getProgress(f *datacapture.File) (int, error) {
	progressFilePath := pt.getProgressFilePath(f)
	//nolint:gosec
	bs, err := ioutil.ReadFile(progressFilePath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, pt.createProgressFile(f)
	}
	if err != nil {
		return 0, err
	}
	return bytesToInt(bs)
}

func bytesToInt(bs []byte) (int, error) {
	i, err := strconv.Atoi(string(bs))
	if err != nil {
		return 0, err
	}
	return i, nil
}

func (pt *progressTracker) createProgressFile(f *datacapture.File) error {
	err := os.WriteFile(pt.getProgressFilePath(f), []byte("0"), os.FileMode(0o777))
	if err != nil {
		return err
	}
	return nil
}

func (pt *progressTracker) deleteProgressFile(f *datacapture.File) error {
	return os.Remove(pt.getProgressFilePath(f))
}

func (pt *progressTracker) updateProgress(f *datacapture.File, requestsWritten int) error {
	i, err := pt.getProgress(f)
	if err != nil {
		return err
	}

	return os.WriteFile(pt.getProgressFilePath(f), []byte(strconv.Itoa(i+requestsWritten)), os.FileMode(0o777))
}

func (pt *progressTracker) getProgressFilePath(f *datacapture.File) string {
	return filepath.Join(pt.progressDir, filepath.Base(f.GetPath()))
}

// Create progress directory in filesystem if it does not already exist.
func (pt *progressTracker) initProgressDir() error {
	_, err := os.Stat(pt.progressDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(pt.progressDir, os.ModePerm); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
