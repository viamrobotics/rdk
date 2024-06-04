package datasync

import "time"

type noopManager struct{}

var _ Manager = (*noopManager)(nil)

// NewNoopManager returns a noop sync manager that does nothing.
func NewNoopManager() Manager {
	return &noopManager{}
}

func (m *noopManager) SyncFile(path string, stopAfter time.Time) {}

func (m *noopManager) SetArbitraryFileTags(tags []string) {}

func (m *noopManager) Close() {}

func (m *noopManager) MarkInProgress(path string) bool {
	return true
}

func (m *noopManager) UnmarkInProgress(path string) {}
