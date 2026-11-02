package cli

import (
	"runtime"
	"testing"

	"golang.org/x/sys/unix"
)

// enforceFilePermissions drops CAP_DAC_OVERRIDE from the current thread's effective capability set
// so file mode bits are enforced even when the test runs as root. Capabilities are per-thread on
// Linux, so the goroutine is locked to its thread and deliberately never unlocked: when the test
// goroutine exits, the runtime terminates the thread rather than reusing it with reduced capabilities.
// If the thread does not hold CAP_DAC_OVERRIDE, permissions are already enforced and nothing changes.
func enforceFilePermissions(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()

	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("capget: %v", err)
	}
	const dacOverrideBit = uint32(1) << (unix.CAP_DAC_OVERRIDE % 32)
	if data[unix.CAP_DAC_OVERRIDE/32].Effective&dacOverrideBit == 0 {
		runtime.UnlockOSThread()
		return
	}

	t.Log("detected test running with CAP_DAC_OVERRIDE, probably as root. locking os thread and dropping capability")
	data[unix.CAP_DAC_OVERRIDE/32].Effective &^= dacOverrideBit
	if err := unix.Capset(&hdr, &data[0]); err != nil {
		t.Fatalf("capset: %v", err)
	}
}
