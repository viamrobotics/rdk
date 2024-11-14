package sync

import "sync"

type fileTracker struct {
	mu    sync.Mutex
	store map[string]bool
}

func newFileTracker() *fileTracker {
	return &fileTracker{store: make(map[string]bool)}
}

// MarkInProgress marks path as in progress in s.inProgress. It returns true if it changed the progress status,
// or false if the path was already in progress.
func (ft *fileTracker) markInProgress(path string) bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	if ft.store[path] {
		return false
	}
	ft.store[path] = true
	return true
}

// InProgress returns true when the file is in progress and false otherwise.
func (ft *fileTracker) inProgress(path string) bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.store[path]
}

// UnmarkInProgress unmarks a path as in progress in s.inProgress.
func (ft *fileTracker) unmarkInProgress(path string) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	delete(ft.store, path)
}
