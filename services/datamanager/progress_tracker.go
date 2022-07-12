package datamanager

import (
	"sync"

	v1 "go.viam.com/api/proto/viam/datasync/v1"
)

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

// Create progress file that stores file upload information.
func (pt *progressTracker) createProgressFile(path string, md *v1.UploadMetadata) error { return nil }

// Delete progress file that stores file upload information.
func (pt *progressTracker) deleteProgressFile(path string) error { return nil }

// Update progress file that stores file upload information with the next sensordata message index to be uploaded.
func (pt *progressTracker) updateIndexProgressFile(path string) error { return nil }

// Returns the index of next sensordata message to upload or -1 if the file upload has not been attempted.
func (pt *progressTracker) getIndexProgressFile(path string) int { return 0 }
