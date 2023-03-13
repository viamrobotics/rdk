package datasync

import "fmt"

type noopManager struct{}

var _ Manager = (*noopManager)(nil)

// NewNoopManager returns a noop sync manager that does nothing.
func NewNoopManager() Manager {
	return &noopManager{}
}

func (m *noopManager) SyncDirectory(dir string) {
	fmt.Printf("[no-op] Fake syncing %s\n", dir)
}

func (m *noopManager) Close() {
	fmt.Println("[no-op] Fake closing")
}
