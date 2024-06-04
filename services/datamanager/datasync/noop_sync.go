package datasync

import "time"

type noopManager struct{}

var _ Manager = (*noopManager)(nil)

// NewNoopManager returns a noop sync manager that does nothing.
func NewNoopManager() Manager {
	return &noopManager{}
}

func (m *noopManager) SyncFiles(_ chan string, _ time.Time) {}

func (m *noopManager) SetArbitraryFileTags(_ []string) {}

func (m *noopManager) Close() {}

func (m *noopManager) MarkInProgress(_ string) bool {
	return true
}

func (m *noopManager) UnmarkInProgress(_ string) {}
