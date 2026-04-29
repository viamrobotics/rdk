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
// using -mode bincirc with a MaxSizeMB cap.
type logmanSessionController struct {
	name         string
	providerGUID string
	outputPath   string
	maxSizeMB    int
}

// Start ensures any previous session with the same name is stopped, then
// creates a fresh session writing to OutputPath. Stop-first is what prevents
// session leaks across crashes: an orphaned session from the prior run gets
// cleared before we try to create ours.
func (l *logmanSessionController) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(l.outputPath), 0o755); err != nil {
		return fmt.Errorf("create etl dir: %w", err)
	}

	runLogman := func(args ...string) error {
		cmdCtx, cancel := context.WithTimeout(ctx, logmanTimeout)
		defer cancel()
		return exec.CommandContext(cmdCtx, "logman", args...).Run()
	}

	// Best-effort: stop any leftover session with this name. Ignore errors —
	// if no session exists the command exits non-zero and we proceed.
	_ = runLogman("stop", l.name, "-ets")

	if err := runLogman("create", "trace", l.name,
		"-p", bracedGUID(l.providerGUID),
		"-o", l.outputPath,
		"-mode", "bincirc",
		"-max", strconv.Itoa(l.maxSizeMB),
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
