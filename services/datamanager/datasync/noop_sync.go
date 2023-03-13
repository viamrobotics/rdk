package datasync

type noopManager struct{}

var _ Manager = (*noopManager)(nil)

// NewNoopManager returns a noop sync manager that does nothing.
func NewNoopManager() Manager {
	return &noopManager{}
}

func (m *noopManager) SyncDirectory(dir string) {}

func (m *noopManager) Close() {}
