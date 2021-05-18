package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

// TestSetupGlobalCache usage implies this test package can *not*
// have any tests run in parallel without risking incorrect usage
// of the global cache. Be advised.
func TestSetupGlobalCache(t *testing.T) (string, func()) {
	globalCacheSingletonMu.Lock()
	globalCacheSingleton = nil
	globalCacheSingletonMu.Unlock()
	cwd, err := os.Getwd()
	test.That(t, err, test.ShouldBeNil)
	dir := t.TempDir()
	startAt := filepath.Join(dir, "one", "two", "three")
	test.That(t, os.MkdirAll(startAt, 0755), test.ShouldBeNil)
	test.That(t, os.Chdir(startAt), test.ShouldBeNil)
	return startAt, func() {
		test.That(t, os.Chdir(cwd), test.ShouldBeNil)
	}
}
