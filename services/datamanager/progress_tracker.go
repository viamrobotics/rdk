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

type progressTracker struct {
	lock *sync.Mutex
	m    map[string]struct{}
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

// Create progress file that stores file upload information.
func (pt *progressTracker) createProgressFile(path string, progress int) error {
	err := ioutil.WriteFile(path, intToBytes(progress), os.FileMode((0o777)))
	if err != nil {
		return err
	}
	return nil
}

// Delete progress file that stores file upload information.
func (pt *progressTracker) deleteProgressFile(path string) error {
	return os.Remove(path)
}

// Update progress file that stores file upload information with the next sensordata message index to be uploaded.
func (pt *progressTracker) updateIndexProgressFile(path string) error {
	i, err := pt.getIndexProgressFile(path)
	if err != nil {
		return err
	}
	if err = pt.createProgressFile(path, i+1); err != nil {
		return err
	}
	return nil
}

// Returns the index of next sensordata message to upload or -1 if the file upload has not been attempted.
func (pt *progressTracker) getIndexProgressFile(path string) (int, error) {
	bs, err := ioutil.ReadFile(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return bytesToInt(bs)
}
