package datamanager

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/pkg/errors"
)

var progressDir = "progress_dir"

// ProgressTracker is responsible for on-disk and in-memory progress tracking.
type ProgressTracker struct {
	lock *sync.Mutex
	m    map[string]struct{}
}

func (pt *ProgressTracker) inProgress(k string) bool {
	pt.lock.Lock()
	defer pt.lock.Unlock()
	_, ok := pt.m[k]
	return ok
}

func (pt *ProgressTracker) mark(k string) {
	pt.lock.Lock()
	pt.m[k] = struct{}{}
	pt.lock.Unlock()
}

func (pt *ProgressTracker) unmark(k string) {
	pt.lock.Lock()
	delete(pt.m, k)
	pt.lock.Unlock()
}

func intToBytes(i int) []byte {
	return []byte(strconv.Itoa(i))
}

func bytesToInt(bs []byte) (int, error) {
	i, err := strconv.Atoi(string(bs))
	if err != nil {
		return 0, err
	}
	return i, nil
}

func (pt *ProgressTracker) createProgressFile(path string, progress int) error {
	err := ioutil.WriteFile(path, intToBytes(progress), os.FileMode((0o777)))
	if err != nil {
		return err
	}
	return nil
}

func (pt *ProgressTracker) deleteProgressFile(path string) error {
	return os.Remove(path)
}

// Increment progress index in progress file.
func (pt *ProgressTracker) updateIndexProgressFile(path string) error {
	i, err := pt.getIndexProgressFile(path)
	if err != nil {
		return err
	}
	if err = pt.createProgressFile(path, i+1); err != nil {
		return err
	}
	return nil
}

// Returns the index of next sensordata message to upload or zero (if no sensordata messages are yet updated).
func (pt *ProgressTracker) getIndexProgressFile(path string) (int, error) {
	bs, err := ioutil.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return bytesToInt(bs)
}
