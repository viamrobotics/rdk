package testutils

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"go.uber.org/multierr"
	v1 "go.viam.com/api/app/datasync/v1"

	"go.viam.com/rdk/utils"
)

// BuildTempModule will run "go build ." in the provided RDK directory and return
// the path to the built temporary file and any build related errors.
func BuildTempModule(tb testing.TB, dir string) (string, error) {
	tb.Helper()
	modPath := filepath.Join(tb.TempDir(), filepath.Base(dir))
	//nolint:gosec
	builder := exec.Command("go", "build", "-o", modPath, ".")
	builder.Dir = utils.ResolveFile(dir)
	out, err := builder.CombinedOutput()
	if len(out) != 0 || err != nil {
		return modPath, multierr.Combine(err, fmt.Errorf(`output from "go build .": %s`, out))
	}
	return modPath, nil
}

// MockBuffer is a buffered writer that just appends data to an array to read
// without needing a real file system for testing.
type MockBuffer struct {
	lock   sync.Mutex
	Writes []*v1.SensorData
}

// Write adds a collected sensor reading to the array.
func (m *MockBuffer) Write(item *v1.SensorData) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.Writes = append(m.Writes, item)
	return nil
}

// Flush does nothing in this implementation as all data will be stored in memory.
func (m *MockBuffer) Flush() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	return nil
}

// Path returns a hardcoded fake path.
func (m *MockBuffer) Path() string {
	return "mock dir"
}

// Length gets the length of the buffer without race conditions.
func (m *MockBuffer) Length() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.Writes)
}
