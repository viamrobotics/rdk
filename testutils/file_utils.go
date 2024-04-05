package testutils

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	v1 "go.viam.com/api/app/datasync/v1"

	"go.viam.com/rdk/utils"
)

// BuildTempModule will run "go build ." in the provided RDK directory and return the
// path to the built temporary file. This function will fail the current test if there
// are any build-related errors.
func BuildTempModule(tb testing.TB, dir string) string {
	tb.Helper()
	modPath := filepath.Join(tb.TempDir(), filepath.Base(dir))
	//nolint:gosec
	builder := exec.Command("go", "build", "-o", modPath, ".")
	builder.Dir = utils.ResolveFile(dir)
	out, err := builder.CombinedOutput()
	// NOTE (Nick S): Workaround for the tickets below:
	// https://viam.atlassian.net/browse/RSDK-7145
	// https://viam.atlassian.net/browse/RSDK-7144
	// Don't fail build if platform has known compile warnings (due to C deps)
	isPlatformWithKnownCompileWarnings := runtime.GOARCH == "arm" || runtime.GOOS == "darwin"
	hasCompilerWarnings := len(out) != 0
	if hasCompilerWarnings && !isPlatformWithKnownCompileWarnings {
		tb.Errorf(`output from "go build .": %s`, out)
	}
	if err != nil {
		tb.Error(err)
	}
	if tb.Failed() {
		tb.Fatalf("failed to build temporary module for testing")
	}
	return modPath
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
	return "/mock/dir"
}

// Length gets the length of the buffer without race conditions.
func (m *MockBuffer) Length() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.Writes)
}
