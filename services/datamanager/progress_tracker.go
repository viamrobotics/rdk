package datamanager

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/pkg/errors"
)

var progressDir = filepath.Join(viamCaptureDotDir, ".progress/")

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

func bytesToInt(bs []byte) (int, error) {
	i, err := strconv.Atoi(string(bs))
	if err != nil {
		return 0, err
	}
	return i, nil
}

func (pt *progressTracker) createProgressFile(path string) error {
	err := ioutil.WriteFile(path, []byte("0"), os.FileMode((0o777)))
	if err != nil {
		return err
	}
	return nil
}

func (pt *progressTracker) deleteProgressFile(path string) error {
	return os.Remove(path)
}

// Increment progress index in progress file.
func (pt *progressTracker) updateProgressFileIndex(path string) error {
	i, err := pt.getProgressFileIndex(path)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path, []byte(strconv.Itoa(i+1)), os.FileMode((0o777)))
	if err != nil {
		return err
	}
	return nil
}

// Returns the index of next sensordata message to upload.
func (pt *progressTracker) getProgressFileIndex(path string) (int, error) {
	bs, err := ioutil.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return bytesToInt(bs)
}

// Create progress directory in filesystem if it does not already exist.
func (pt *progressTracker) initProgressDir() error {
	if _, err := os.Stat(pt.progressDir); os.IsNotExist(err) {
		if err := os.MkdirAll(pt.progressDir, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}
