package testutils

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/utils"
)

const osDarwin = "darwin"

type mutexMap struct {
	sync.Mutex
	mutexes map[string]*sync.Mutex
}

// safely retrieve or create the mutex for the given key.
func (mm *mutexMap) get(key string) *sync.Mutex {
	mm.Lock()
	defer mm.Unlock()
	mut, ok := mm.mutexes[key]
	if !ok {
		mut = &sync.Mutex{}
		mm.mutexes[key] = mut
	}
	return mut
}

// map of path => mutex. prevents overlapping builds, opening the door to
// test parallelism. todo: test whether this is actually necessary.
var buildMutex = mutexMap{mutexes: make(map[string]*sync.Mutex)}

// BuildViamServer will attempt to build the viam-server (server-static if on linux). If successful, this function will
// return the path to the executable.
func BuildViamServer(tb testing.TB) string {
	tb.Helper()

	buildOutputPath := tb.TempDir()
	serverPath := filepath.Join(buildOutputPath, "viam-server-static")

	var builder *exec.Cmd

	if runtime.GOOS != "windows" {
		command := "server-static"
		if runtime.GOOS == osDarwin {
			command = "server"
			serverPath = filepath.Join(buildOutputPath, "viam-server")
		}
		builder = exec.Command("make", command)
		builder.Env = append(os.Environ(), "TESTBUILD_OUTPUT_PATH="+buildOutputPath)
	} else {
		// we don't have access to make on Windows, so copy the build command from the Makefile.
		serverPath += ".exe"
		//nolint:gosec
		builder = exec.Command(
			"go", "build", "-tags", "no_cgo,osusergo,netgo",
			"-ldflags=-extldflags=-static -s -w",
			"-o", serverPath,
			"./web/cmd/server",
		)
	}
	// set Dir to root of repo
	builder.Dir = utils.ResolveFile(".")
	out, err := builder.CombinedOutput()
	if len(out) > 0 {
		tb.Logf("Build Output: %s", out)
	}
	if err != nil {
		tb.Error(err)
	}
	if tb.Failed() {
		tb.Fatal("failed to build viam-server executable")
	}
	return serverPath
}

// length `n` truncated md5sum of `input` string.
func shortHash(input string, n int) (string, error) {
	hash := md5.New() //nolint:gosec
	_, err := hash.Write([]byte(input))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil))[:n], nil
}

// takes a path (presumably in a stable shared location), returns a symlink to it
// in a t.TempDir.
func symlinkTempDir(tb testing.TB, realPath string) string {
	// We have this because some tests save things in the module folder,
	// others rely on this folder being unique for each invocation.
	tb.Helper()
	linkPath := filepath.Join(tb.TempDir(), filepath.Base(realPath))
	test.That(tb, os.Symlink(realPath, linkPath), test.ShouldBeNil)
	return linkPath
}

// BuildTempModule will attempt to build the module in the provided directory and put the
// resulting executable binary into a temporary directory. If successful, this function will
// return the path to the executable binary.
func BuildTempModule(tb testing.TB, modDir string) string {
	tb.Helper()

	// todo: cross-test-process locking instead of per-package
	mut := buildMutex.get(modDir)
	mut.Lock()
	defer mut.Unlock()

	// todo: hash the entire file tree under modDir, and do this above buildMutex
	dirHash, err := shortHash(modDir, 8)
	test.That(tb, err, test.ShouldBeNil)
	// todo: clean this up at the beginning and end of each test run
	// exePath is a stable temporary location for this temp module; it will be
	// reused by all tests running in this process.
	exePath := filepath.Join(os.TempDir(), "rdk-build", strconv.Itoa(os.Getpid()),
		dirHash, filepath.Base(modDir))
	if runtime.GOOS == "windows" {
		exePath += ".exe"
	}
	if _, err := os.Stat(exePath); err == nil {
		// it exists, reusing
		return symlinkTempDir(tb, exePath)
	}

	//nolint:gosec
	builder := exec.Command("go", "build", "-o", exePath, ".")
	builder.Dir = utils.ResolveFile(modDir)
	out, err := builder.CombinedOutput()
	// NOTE (Nick S): Workaround for the tickets below:
	// https://viam.atlassian.net/browse/RSDK-7145
	// https://viam.atlassian.net/browse/RSDK-7144
	// Don't fail build if platform has known compile warnings (due to C deps)
	isPlatformWithKnownCompileWarnings := runtime.GOARCH == "arm" || runtime.GOOS == osDarwin
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
	return symlinkTempDir(tb, exePath)
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

// VerifyDirectoryBuilds verifies that the Go code in the specified directory builds successfully.
// Warnings are allowed; only build failures cause the test to fail.
func VerifyDirectoryBuilds(tb testing.TB, dir string) {
	tb.Helper()

	resolvedDir := utils.ResolveFile(dir)
	//nolint:gosec
	builder := exec.Command("go", "build", "-o", os.DevNull, ".")
	builder.Dir = resolvedDir
	out, err := builder.CombinedOutput()
	if err != nil {
		// Build failed - show detailed compiler output
		tb.Fatalf("failed to build directory %s:\nError: %v\n\nCompiler output:\n%s", dir, err, string(out))
	}
}

// MockBuffer is a buffered writer that just appends data to an array to read
// without needing a real file system for testing.
type MockBuffer struct {
	t      *testing.T
	ctx    context.Context
	cancel context.CancelFunc
	Writes chan []*v1.SensorData
}

// NewMockBuffer returns a mock buffer.
// This needs to be closed before the collector, otherwise the
// collector's Close method will block.
func NewMockBuffer(t *testing.T) *MockBuffer {
	c, cancel := context.WithCancel(context.Background())
	return &MockBuffer{
		t:      t,
		ctx:    c,
		cancel: cancel,
		Writes: make(chan []*v1.SensorData, 1),
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
	writes chan []*v1.SensorData,
	expecteds []*v1.SensorData,
) {
	select {
	case <-ctx.Done():
		t.Error("timeout")
		t.FailNow()
	case writes := <-writes:
		end := time.Now()
		test.That(t, len(writes), test.ShouldEqual, len(expecteds))
		for i, expected := range expecteds {
			write := writes[i]
			requestedAt := write.Metadata.TimeRequested.AsTime()
			receivedAt := write.Metadata.TimeReceived.AsTime()
			test.That(t, start, test.ShouldHappenOnOrBefore, requestedAt)
			test.That(t, requestedAt, test.ShouldHappenOnOrBefore, receivedAt)
			test.That(t, receivedAt, test.ShouldHappenOnOrBefore, end)
			test.That(t, len(expecteds), test.ShouldEqual, len(writes))
			// nil out to make comparable
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
}

// Close cancels the MockBuffer context so all methods stop blocking.
func (m *MockBuffer) Close() {
	m.cancel()
}

// WriteBinary writes binary sensor data.
func (m *MockBuffer) WriteBinary(item *v1.SensorData, mimeType string) error {
	if err := m.ctx.Err(); err != nil {
		return err
	}

	if !isBinary(item) {
		m.t.Errorf("MockBuffer.WriteBinary called with non binary data. item: %#v\n", item)
		m.t.FailNow()
	}

	select {
	case m.Writes <- []*v1.SensorData{item}:
	case <-m.ctx.Done():
	}
	return nil
}

// WriteTabular writes tabular sensor data to the Writes channel.
func (m *MockBuffer) WriteTabular(item *v1.SensorData) error {
	if err := m.ctx.Err(); err != nil {
		return err
	}
	if isBinary(item) {
		m.t.Errorf("MockBuffer.WriteTabular called with binary data. item: %#v\n", item)
		m.t.FailNow()
	}
	select {
	case m.Writes <- []*v1.SensorData{item}:
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
