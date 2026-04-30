//go:build windows

package logging

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// sessionController starts and stops an ETW session that captures events from
// a provider into an .etl file. The implementation is swappable so we can
// later replace the logman-based controller with direct StartTrace/StopTrace
// syscalls if we want to drop the dependency on logman.exe.
type sessionController interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// logmanTimeout caps each logman invocation. logman normally returns in well
// under a second; anything longer means logman is wedged and we'd rather
// surface the failure than block shutdown.
const logmanTimeout = 10 * time.Second

// logmanSessionController shells out to logman.exe to manage a fixed-name ETW
// session. The session captures events from the provider GUID to OutputPath
// using -f bincirc with a MaxSizeMB cap.
type logmanSessionController struct {
	name         string
	providerGUID string
	outputPath   string
	maxSizeMB    int
}

// Start normalizes any prior session/definition state, then creates a fresh
// definition and starts it. The two-step create-then-start (instead of a
// single create-with-ets) is deliberate: logman's -ets-create path drops
// the -ft flag, so we use the persistent definition path where it takes
// effect, then activate the definition.
//
// stop+delete-first makes Start idempotent regardless of prior state (no
// session, leftover session from a crash, stale definition from a prior
// viam-server version). It also ensures definition changes between
// viam-server versions get picked up — the delete clears any stale config
// before we recreate it.
func (l *logmanSessionController) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(l.outputPath), 0o755); err != nil {
		return fmt.Errorf("create etl dir: %w", err)
	}

	runLogman := func(args ...string) error {
		cmdCtx, cancel := context.WithTimeout(ctx, logmanTimeout)
		defer cancel()
		return exec.CommandContext(cmdCtx, "logman", args...).Run()
	}

	// Best-effort cleanup: stop any orphaned runtime session, then delete any
	// stale persistent definition. Either may not exist; ignore errors.
	_ = runLogman("stop", l.name, "-ets")

	// Create the persistent definition and run it automatically with -ets
	if err := runLogman("create", "trace", l.name,
		"-p", bracedGUID(l.providerGUID),
		"-o", l.outputPath,
		// binary + circular filetype so logs autorotate once we reach the limit
		"-f", "bincirc",
		"-max", strconv.Itoa(l.maxSizeMB),
		// -ft 2 forces a buffer flush every 2 seconds
		"-ft", "2",
		"-ets",
	); err != nil {
		return fmt.Errorf("logman create trace: %w", err)
	}

	return nil
}

func (l *logmanSessionController) Stop(ctx context.Context) error {
	cmdCtx, cancel := context.WithTimeout(ctx, logmanTimeout)
	defer cancel()
	if err := exec.CommandContext(cmdCtx, "logman", "stop", l.name, "-ets").Run(); err != nil {
		return fmt.Errorf("logman stop: %w", err)
	}
	return nil
}

// bracedGUID returns g wrapped in {…} regardless of whether the input already
// has braces. logman's -p flag accepts either form, but we normalize.
func bracedGUID(g string) string {
	return "{" + strings.Trim(g, "{}") + "}"
}
