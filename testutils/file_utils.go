package testutils

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	v1 "go.viam.com/api/app/datasync/v1"

	"go.viam.com/rdk/utils"
)

// BuildTempModule will attempt to build the module in the provided directory and put the
// resulting executable binary into a temporary directory. If successful, this function will
// return the path to the executable binary.
func BuildTempModule(tb testing.TB, modDir string) string {
	// tb.Helper()
	exePath := filepath.Join(tb.TempDir(), filepath.Base(modDir))
	//nolint:gosec
	builder := exec.Command("go", "build", "-o", exePath, ".")
	builder.Dir = utils.ResolveFile(modDir)
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
	return exePath
}

// BuildTempModuleWithFirstRun will attempt to build the module in the provided directory and put the
// resulting executable binary into a temporary directory. After building, it will also copy "meta.json"
// and "first_run.sh" into the same temporary directory. It is assumed that these files are in the
// provided module directory. If successful, this function will return the path to the executable binary.
func BuildTempModuleWithFirstRun(tb testing.TB, modDir string) string {
	// tb.Helper()

	exePath := BuildTempModule(tb, modDir)
	exeDir := filepath.Dir(exePath)

	type copyOp struct {
		src string
		dst string
	}

	for _, cp := range []copyOp{
		// TODO(RSDK-9151): Having a `meta.json` in the testmodule directory results in
		// unintended behavior changes across our test suite. To avoid this, we name the
		// file `first_run_meta.json` in the source test module directory and rename it in the
		// destination directory that contains the built executable. This is fine for now
		// but we should investigate and understand why this happens.
		{"first_run_meta.json", "meta.json"},
		{"first_run.sh", "first_run.sh"},
	} {
		srcPath := utils.ResolveFile(filepath.Join(modDir, cp.src))
		//nolint:gosec
		src, err := os.Open(srcPath)
		if err != nil {
			tb.Fatal(err)
		}
		dstPath := filepath.Join(exeDir, cp.dst)
		//nolint:gosec
		dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o500)
		if err != nil {
			tb.Fatal(err)
		}
		closeFiles := func() {
			if err := errors.Join(src.Close(), dst.Close()); err != nil {
				tb.Error(err)
			}
		}
		if _, err = io.Copy(dst, src); err != nil {
			closeFiles()
			tb.Fatal(err)
		}
		closeFiles()
		if tb.Failed() {
			tb.FailNow()
		}
	}
	return exePath
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
