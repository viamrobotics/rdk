package testutils

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/utils"
)

// BuildTempModule will attempt to build the module in the provided directory and put the
// resulting executable binary into a temporary directory. If successful, this function will
// return the path to the executable binary.
func BuildTempModule(tb testing.TB, modDir string) string {
	tb.Helper()

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
	tb.Helper()

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
	ctx    context.Context
	cancel context.CancelFunc
	Writes chan *v1.SensorData
}

// NewMockBuffer returns a mock buffer.
// This needs to be closed before the collector, otherwise the
// collector's Close method will block.
func NewMockBuffer() *MockBuffer {
	c, cancel := context.WithCancel(context.Background())
	return &MockBuffer{
		ctx:    c,
		cancel: cancel,
		Writes: make(chan *v1.SensorData, 1),
	}
}

// ToStructPBStruct calls structpb.NewValue and fails tests if an error
// is encountered.
// Otherwise, returns a *structpb.Struct.
func ToStructPBStruct(t *testing.T, v any) *structpb.Struct {
	s, err := structpb.NewValue(v)
	test.That(t, err, test.ShouldBeNil)
	return s.GetStructValue()
}

func isBinary(item *v1.SensorData) bool {
	if item == nil {
		return false
	}
	switch item.Data.(type) {
	case *v1.SensorData_Binary:
		return true
	default:
		return false
	}
}

// CheckMockBufferWrites checks that the Write
// match the expected data & metadata (timestamps).
func CheckMockBufferWrites(
	t *testing.T,
	ctx context.Context,
	start time.Time,
	writes chan *v1.SensorData,
	expected *v1.SensorData,
) {
	select {
	case <-ctx.Done():
		t.Error("timeout")
		t.FailNow()
	case write := <-writes:
		end := time.Now()
		// nil out to make comparable
		requestedAt := write.Metadata.TimeRequested.AsTime()
		receivedAt := write.Metadata.TimeReceived.AsTime()
		test.That(t, start, test.ShouldHappenOnOrBefore, requestedAt)
		test.That(t, requestedAt, test.ShouldHappenOnOrBefore, receivedAt)
		test.That(t, receivedAt, test.ShouldHappenOnOrBefore, end)
		// nil out to make comparable
		write.Metadata.TimeRequested = nil
		write.Metadata.TimeReceived = nil
		test.That(t, write.GetMetadata(), test.ShouldResemble, expected.GetMetadata())
		if isBinary(write) {
			test.That(t, write.GetBinary(), test.ShouldResemble, expected.GetBinary())
		} else {
			test.That(t, write.GetStruct(), test.ShouldResemble, expected.GetStruct())
		}
	}
}

// Close cancels the MockBuffer context so all methods stop blocking.
func (m *MockBuffer) Close() {
	m.cancel()
}

// Write adds the item to the channel.
func (m *MockBuffer) Write(item *v1.SensorData) error {
	if err := m.ctx.Err(); err != nil {
		return err
	}
	select {
	case m.Writes <- item:
	case <-m.ctx.Done():
	}
	return nil
}

// Flush does nothing in this implementation as all data will be stored in memory.
func (m *MockBuffer) Flush() error {
	return nil
}

// Path returns a hardcoded fake path.
func (m *MockBuffer) Path() string {
	return "/mock/dir"
}
