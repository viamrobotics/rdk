//go:build windows

package winlogproc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"braces.dev/errtrace"
	"go.viam.com/rdk/logging"
)

// CollectOpts configures Collect. Zero values fall back to defaults
// that match viam-server's production behavior.
type CollectOpts struct {
	// ETLDir is the directory containing per-session .etl files
	// produced by the ETW logger. Collect globs *.etl inside and feeds
	// every match to tracerpt in one invocation. Default:
	// DefaultETLDir().
	ETLDir string

	// SessionName is the ETW session whose buffers Collect flushes
	// before reading the .etl files. Empty means skip the flush.
	SessionName string

	// EventlogSource is the classic Application Event Log source.
	// Empty disables the eventlog dump.
	EventlogSource string

	// OutDir is where raw dumps and the processed/ subdir land. Empty
	// means a fresh ./winlogs-<timestamp>/ in the cwd.
	OutDir string

	// After and Before optionally bound the Get-EventLog dump window
	// and the post-tracerpt trace.tsv filter. Zero values mean
	// unbounded.
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
	if opts.ETLDir == "" {
		opts.ETLDir = DefaultETLDir()
	}
	if opts.SessionName == "" {
		opts.SessionName = logging.ServerETW.SessionName
	}
	if opts.EventlogSource == "" {
		opts.EventlogSource = logging.ServerETW.ProviderName
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return "", errtrace.Wrap(fmt.Errorf("mkdir out: %w", err))
	}

	rawTrace := filepath.Join(opts.OutDir, "trace.xml")
	rawEventlog := filepath.Join(opts.OutDir, "eventlog.txt")

	etlFiles, err := filepath.Glob(filepath.Join(opts.ETLDir, "*.etl"))
	if err != nil {
		return "", errtrace.Wrap(fmt.Errorf("glob %s: %w", opts.ETLDir, err))
	}
	if len(etlFiles) == 0 {
		return "", errtrace.Wrap(fmt.Errorf("no .etl files found in %s", opts.ETLDir))
	}

	// Echo the resolved paths and parameters before we touch them so
	// failures can be diagnosed without re-running with extra logging.
	fmt.Fprintln(os.Stderr, "winlogproc.Collect:")
	fmt.Fprintln(os.Stderr, "  ETL dir         :", opts.ETLDir)
	fmt.Fprintln(os.Stderr, "  ETL files       :", len(etlFiles), "found")
	for _, f := range etlFiles {
		fmt.Fprintln(os.Stderr, "    ", f)
	}
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

	// tracerpt accepts multiple .etl inputs; merge all retained
	// session files into one trace.xml.
	args := append([]string{}, etlFiles...)
	args = append(args, "-o", rawTrace, "-of", "XML", "-y")
	if out, err := exec.Command("tracerpt", args...).CombinedOutput(); err != nil {
		return "", errtrace.Wrap(fmt.Errorf("tracerpt: %w: %s", err, out))
	}

	if err := dumpEventlog(rawEventlog, opts.EventlogSource, opts.After, opts.Before); err != nil {
		return "", errtrace.Wrap(err)
	}

	processedDir := filepath.Join(opts.OutDir, "processed")
	if err := os.MkdirAll(processedDir, 0o755); err != nil {
		return "", errtrace.Wrap(err)
	}
	if err := Eventlog(rawEventlog, filepath.Join(processedDir, "eventlog.tsv")); err != nil {
		return "", errtrace.Wrap(fmt.Errorf("processing eventlog: %w", err))
	}
	if err := Trace(rawTrace, filepath.Join(processedDir, "trace.tsv"), opts.After, opts.Before); err != nil {
		return "", errtrace.Wrap(fmt.Errorf("processing trace: %w", err))
	}

	return opts.OutDir, nil
}

// DefaultETLDir returns the directory viam-server writes per-session
// .etl files into under its production defaults — <VIAM_HOME>/logs,
// with VIAM_HOME falling back to /opt/viam.
func DefaultETLDir() string {
	home := os.Getenv("VIAM_HOME")
	if home == "" {
		if _, err := os.UserHomeDir(); err == nil {
			home = filepath.Join("/opt", "viam")
		}
	}
	return filepath.Join(home, "logs")
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
		return errtrace.Wrap(fmt.Errorf("Get-EventLog failed: %w: %s", err, out))
	}
	return nil
}
