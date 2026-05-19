//go:build windows

package logging

import (
	"path/filepath"
	"testing"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/go-winio/pkg/guid"
	"go.viam.com/test"
)

// TestETWAppenderLifecycle exercises Write/Close/Write-after-close on the
// appender directly (no session). Doesn't depend on logman or admin
// privileges — provider registration is unprivileged.
func TestETWAppenderLifecycle(t *testing.T) {
	g, err := guid.FromString(providerGUID)
	test.That(t, err, test.ShouldBeNil)

	provider, err := etw.NewProviderWithID("viam-server-test", g, nil)
	test.That(t, err, test.ShouldBeNil)

	a := &etwAppender{provider: provider, session: nil}

	logger := NewLogger("etw-test")
	logger.AddAppender(a)

	logger.Info("hello")
	logger.Info("contains \x00 a null")
	logger.Warn("warn level")
	logger.Error("error level")

	test.That(t, a.Close(), test.ShouldBeNil)

	// Write after close: provider is unregistered, pkg/etw's WriteEvent checks
	// provider.enabled and no-ops. Must not panic.
	logger.Info("after close")
}

// TestRegisterETWLogger covers the public path including session start.
// Session start may fail (no admin / no logman in PATH) — that's handled
// internally and the closer is still returned.
func TestRegisterETWLogger(t *testing.T) {
	logger := NewLogger("etw-register-test")

	etlPath := filepath.Join(t.TempDir(), "test.etl")
	closer, err := RegisterETWLogger(logger, "viam-server-test", etlPath)
	test.That(t, closer, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	logger.Info("message through registered ETW appender")

	// Session stop returns an error when no session was running; we only
	// require no panic here.
	_ = closer.Close()
}
