package datasync

type noopManager struct{}

var _ Manager = (*noopManager)(nil)

// NewNoopManager returns a noop sync manager that does nothing.
func NewNoopManager() Manager {
	return &noopManager{}
}

func (m *noopManager) SyncFile(path string) {}

func (m *noopManager) SetArbitraryFileTags(tags []string) {}

func (m *noopManager) Close() {}
