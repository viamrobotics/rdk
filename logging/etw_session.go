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

// Start creates and starts the ETW session. -ets acts on the session directly
// and saves no data collector definition, so there is nothing for Stop to
// delete afterwards.
//
// The stop-first keeps Start idempotent against a session leaked by a crash.
// Note that session names are global to the machine: starting a controller
// under a name another process is already using will stop that process's
// session out from under it.
func (l *logmanSessionController) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(l.outputPath), 0o755); err != nil {
		return fmt.Errorf("create etl dir: %w", err)
	}

	// Best-effort cleanup: stop any orphaned runtime session. Ignore errors —
	// no session means stop fails harmlessly.
	_ = l.logman(ctx, "stop", l.name, "-ets")

	if err := l.logman(ctx, "create", "trace", l.name,
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
	if err := l.logman(ctx, "stop", l.name, "-ets"); err != nil {
		return fmt.Errorf("logman stop: %w", err)
	}
	return nil
}

func (l *logmanSessionController) logman(ctx context.Context, args ...string) error {
	cmdCtx, cancel := context.WithTimeout(ctx, logmanTimeout)
	defer cancel()
	return exec.CommandContext(cmdCtx, "logman", args...).Run()
}

// bracedGUID returns g wrapped in {…} regardless of whether the input already
// has braces. logman's -p flag accepts either form, but we normalize.
func bracedGUID(g string) string {
	return "{" + strings.Trim(g, "{}") + "}"
}
