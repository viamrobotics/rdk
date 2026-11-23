//go:build !linux

package cli

import (
	"os"
	"testing"
)

// enforceFilePermissions skips the test when running as root, since root bypasses file mode bits
// and dropping that privilege for a single thread is only supported on Linux.
func enforceFilePermissions(t *testing.T) {
	t.Helper()
	if os.Getuid() == 0 {
		t.Skip("root bypasses file permission bits")
	}
}
