package sync

import "sync"

type fileTracker struct {
	mu sync.Mutex
	// store maps an in-progress file path to its progress logger. The value is nil
	// between markInProgress and setProgress (and for uploads that carry no byte
	// progress, e.g. sequences); presence of the key alone denotes "in progress".
	store map[string]*uploadProgressLogger
}

func newFileTracker() *fileTracker {
	return &fileTracker{store: make(map[string]*uploadProgressLogger)}
}

// markInProgress marks path as in progress. It returns true if it changed the progress
// status, or false if the path was already in progress.
func (ft *fileTracker) markInProgress(path string) bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if _, ok := ft.store[path]; ok {
		return false
	}
	ft.store[path] = nil
	return true
}

// inProgress returns true when the file is in progress and false otherwise.
func (ft *fileTracker) inProgress(path string) bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	_, ok := ft.store[path]
	return ok
}

// setProgress records p as the live progress logger for an already-tracked path so that
// progress can be introspected while the upload runs. Paths that are not currently tracked
// (e.g. a direct dataset upload that bypasses the sync-worker dedup) are ignored so we
// don't create a phantom in-progress entry that never gets unmarked. This does not take
// ownership of p's lifecycle: the caller always closes it via a defer.
func (ft *fileTracker) setProgress(path string, p *uploadProgressLogger) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if _, ok := ft.store[path]; ok {
		ft.store[path] = p
	}
}

// unmarkInProgress unmarks a path as in progress. It also closes any attached progress
// logger as a safety net; close is idempotent, so this composes with the caller's own
// deferred close. Closing happens outside the lock since it stops a background goroutine.
func (ft *fileTracker) unmarkInProgress(path string) {
	ft.mu.Lock()
	p := ft.store[path]
	delete(ft.store, path)
	ft.mu.Unlock()
	p.close()
}
