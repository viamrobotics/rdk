//go:build windows

package winlogproc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultSessionName is the ETW session name viam-server uses (matches
// the constant in logging/windows_etw_logger.go).
const DefaultSessionName = "viam-server-trace"

// DefaultEventlogSource is the classic Application Event Log source
// viam-server registers (matches the call in web/server/entrypoint.go).
const DefaultEventlogSource = "viam-server"

// CollectOpts configures Collect. Zero values fall back to defaults
// that match viam-server's production behavior.
type CollectOpts struct {
	// ETLPath is the .etl file the ETW session writes to. Default:
	// <VIAM_HOME>/logs/viam-server-trace.etl, with VIAM_HOME falling
	// back to $USERPROFILE/.viam (matches utils.viamDotDir).
	ETLPath string

	// SessionName is the ETW session whose buffers Collect flushes
	// before reading the .etl file. Empty means skip the flush.
	SessionName string

	// EventlogSource is the classic Application Event Log source.
	// Empty disables the eventlog dump.
	EventlogSource string

	// OutDir is where raw dumps and the processed/ subdir land. Empty
	// means a fresh ./winlogs-<timestamp>/ in the cwd.
	OutDir string

	// After and Before optionally bound the Get-EventLog dump window.
	// Zero values mean unbounded.
	After, Before time.Time
}

// Collect runs the full collect-then-process pipeline:
//
//  1. Flush the named ETW session via `logman update -fd -ets`
//     (best-effort; ignored if no session is running).
//  2. Dump the .etl to <OutDir>/trace.csv via tracerpt.
//  3. Dump the eventlog source to <OutDir>/eventlog.txt via PowerShell
//     Get-EventLog.
//  4. Process both into <OutDir>/processed/{eventlog,trace}.tsv via
//     Eventlog and Trace.
//
// Returns the resolved OutDir on success.
func Collect(opts CollectOpts) (string, error) {
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join(".", "winlogs-"+time.Now().Format("2006-01-02T15-04-05"))
	}
	if opts.ETLPath == "" {
		opts.ETLPath = DefaultETLPath()
	}
	if opts.SessionName == "" {
		opts.SessionName = DefaultSessionName
	}
	if opts.EventlogSource == "" {
		opts.EventlogSource = DefaultEventlogSource
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir out: %w", err)
	}

	rawTrace := filepath.Join(opts.OutDir, "trace.xml")
	rawEventlog := filepath.Join(opts.OutDir, "eventlog.txt")

	// Echo the resolved paths and parameters before we touch them so
	// failures can be diagnosed without re-running with extra logging.
	fmt.Fprintln(os.Stderr, "winlogproc.Collect:")
	fmt.Fprintln(os.Stderr, "  ETL path        :", opts.ETLPath)
	fmt.Fprintln(os.Stderr, "  ETW session     :", opts.SessionName)
	fmt.Fprintln(os.Stderr, "  Eventlog source :", opts.EventlogSource)
	fmt.Fprintln(os.Stderr, "  Out dir         :", opts.OutDir)
	fmt.Fprintln(os.Stderr, "  Raw trace.xml   :", rawTrace)
	fmt.Fprintln(os.Stderr, "  Raw eventlog.txt:", rawEventlog)
	if !opts.After.IsZero() {
		fmt.Fprintln(os.Stderr, "  After           :", opts.After.Format(time.RFC3339Nano))
	}
	if !opts.Before.IsZero() {
		fmt.Fprintln(os.Stderr, "  Before          :", opts.Before.Format(time.RFC3339Nano))
	}

	// Best-effort flush. Fails harmlessly if no session is running
	// (e.g. viam-server already exited and stopped the session).
	_ = exec.Command("logman", "update", opts.SessionName, "-fd", "-ets").Run()

	if out, err := exec.Command("tracerpt", opts.ETLPath, "-o", rawTrace, "-of", "XML", "-y").CombinedOutput(); err != nil {
		return "", fmt.Errorf("tracerpt %s: %w: %s", opts.ETLPath, err, out)
	}

	if err := dumpEventlog(rawEventlog, opts.EventlogSource, opts.After, opts.Before); err != nil {
		return "", err
	}

	processedDir := filepath.Join(opts.OutDir, "processed")
	if err := os.MkdirAll(processedDir, 0o755); err != nil {
		return "", err
	}
	if err := Eventlog(rawEventlog, filepath.Join(processedDir, "eventlog.tsv")); err != nil {
		return "", fmt.Errorf("processing eventlog: %w", err)
	}
	if err := Trace(rawTrace, filepath.Join(processedDir, "trace.tsv"), opts.After, opts.Before); err != nil {
		return "", fmt.Errorf("processing trace: %w", err)
	}

	return opts.OutDir, nil
}

// DefaultETLPath returns the .etl path viam-server writes to under its
// production defaults — <VIAM_HOME>/logs/viam-server-trace.etl, with
// VIAM_HOME falling back to /opt/viam.
func DefaultETLPath() string {
	home := os.Getenv("VIAM_HOME")
	if home == "" {
		if _, err := os.UserHomeDir(); err == nil {
			home = filepath.Join("/opt", "viam")
		}
	}
	return filepath.Join(home, "logs", "viam-server-trace.etl")
}

// dumpEventlog shells to PowerShell to write a Get-EventLog dump in
// the same time<TAB>EntryType<TAB>Message format Eventlog expects.
func dumpEventlog(outPath, source string, after, before time.Time) error {
	var filter strings.Builder
	if !after.IsZero() {
		fmt.Fprintf(&filter, " -After ([datetime]::Parse('%s'))", after.Format(time.RFC3339Nano))
	}
	if !before.IsZero() {
		fmt.Fprintf(&filter, " -Before ([datetime]::Parse('%s'))", before.Format(time.RFC3339Nano))
	}

	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$tab = [char]9
Get-EventLog -LogName Application -Source '%s'%s |
  Sort-Object TimeGenerated |
  ForEach-Object {
    $_.TimeGenerated.ToUniversalTime().ToString("o") + $tab + $_.EntryType + $tab + $_.Message
  } |
  Out-File -FilePath '%s' -Encoding utf8
`, source, filter.String(), outPath)

	out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Get-EventLog failed: %w: %s", err, out)
	}
	return nil
}
