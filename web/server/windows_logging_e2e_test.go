//go:build windows

package server_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"golang.org/x/sys/windows"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

const (
	// runDuration is how long viam-server runs before the test sends
	// Ctrl+Break. Bump this to capture more log volume.
	runDuration = 10 * time.Second

	// shutdownGracePeriod caps how long the test waits for viam-server
	// to exit after Ctrl+Break before force-killing.
	shutdownGracePeriod = 15 * time.Second

	// etwSessionName must match the constant in
	// logging/windows_etw_logger.go.
	etwSessionName = "viam-server-trace"
)

// TestWindowsLoggingE2E builds viam-server, runs it for runDuration with
// both the classic EventLogger and the new ETW appender enabled, then
// dumps each log stream to a separate file for manual comparison.
// No content assertions — this test exists to produce the two files.
//
// Requires Administrator: the ETW session start path in entrypoint.go
// shells out to `logman create trace -ets`, which is privileged. The
// test skips otherwise.
//
// Artifacts (viam-server stdout, dump files, the .etl) land in a
// persistent temp dir whose path is logged via t.Logf — inspect after
// the test completes, delete the dir when done. Run with `go test -v`
// to see the paths.
func TestWindowsLoggingE2E(t *testing.T) {
	if !isElevated() {
		t.Skip("requires Administrator (ETW session start needs logman privileges)")
	}

	logger := logging.NewTestLogger(t)

	// Persistent dir for all artifacts. Not cleaned up automatically.
	e2eDir, err := os.MkdirTemp("", "viam-logging-e2e-")
	test.That(t, err, test.ShouldBeNil)
	t.Logf("artifacts dir: %s", e2eDir)

	port, err := goutils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)

	cfg, err := config.Read(context.Background(), utils.ResolveFile("/etc/configs/fake.json"), logger, nil)
	test.That(t, err, test.ShouldBeNil)
	cfg.Network.BindAddress = fmt.Sprintf(":%d", port)
	cfgPath, err := robottestutils.MakeTempConfig(t, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	serverPath := rtestutils.BuildViamServer(t)

	stdoutPath := filepath.Join(e2eDir, "viam-server.stdout")
	stdoutFile, err := os.Create(stdoutPath)
	test.That(t, err, test.ShouldBeNil)
	defer stdoutFile.Close()

	// CREATE_NEW_PROCESS_GROUP lets us deliver CTRL_BREAK_EVENT to this
	// specific process via GenerateConsoleCtrlEvent. Without it, the
	// event reaches every process sharing the test's console.
	cmd := exec.Command(serverPath, "-config", cfgPath)
	cmd.Env = append(os.Environ(), "VIAM_HOME="+e2eDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Stdout = stdoutFile
	cmd.Stderr = stdoutFile

	startTime := time.Now()
	test.That(t, cmd.Start(), test.ShouldBeNil)

	t.Logf("viam-server pid=%d running for %s", cmd.Process.Pid, runDuration)
	time.Sleep(runDuration)

	// Ctrl+Break — viam-server's signal handler runs the deferred
	// etwCloser.Close in entrypoint.go, which stops the ETW session and
	// flushes kernel buffers into the .etl file before exit.
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid)); err != nil {
		t.Logf("GenerateConsoleCtrlEvent: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case waitErr := <-done:
		t.Logf("viam-server exited: %v", waitErr)
	case <-time.After(shutdownGracePeriod):
		t.Log("viam-server did not exit within grace period; killing")
		_ = cmd.Process.Kill()
		<-done
	}
	endTime := time.Now()

	// Belt-and-braces: if the in-process closer didn't run (force-kill
	// path above), stop the session ourselves. No-op when already
	// stopped.
	_ = exec.Command("logman", "stop", etwSessionName, "-ets").Run()

	// Dump classic Event Log entries within the test's wall-clock
	// window so unrelated viam-server runs on this machine don't
	// bleed in.
	eventlogPath := filepath.Join(e2eDir, "eventlog.txt")
	psScript := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$start = [datetime]::Parse('%s')
$end   = [datetime]::Parse('%s')
$tab = [char]9
Get-EventLog -LogName Application -Source viam-server -After $start -Before $end |
  Sort-Object TimeGenerated |
  ForEach-Object {
    $_.TimeGenerated.ToUniversalTime().ToString("o") + $tab + $_.EntryType + $tab + $_.Message
  } |
  Out-File -FilePath '%s' -Encoding utf8
`, startTime.Format(time.RFC3339Nano), endTime.Format(time.RFC3339Nano), eventlogPath)

	psPath := filepath.Join(e2eDir, "dump-eventlog.ps1")
	test.That(t, os.WriteFile(psPath, []byte(psScript), 0o600), test.ShouldBeNil)

	out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", psPath).CombinedOutput()
	if err != nil {
		t.Logf("powershell output: %s", out)
		t.Fatalf("Get-EventLog dump failed: %v", err)
	}

	// Dump ETW events from the .etl file produced by the session.
	etlPath := filepath.Join(e2eDir, "logs", "viam-server-trace.etl")
	tracerptOut := filepath.Join(e2eDir, "trace.csv")
	out, err = exec.Command("tracerpt", etlPath, "-o", tracerptOut, "-of", "CSV", "-y").CombinedOutput()
	if err != nil {
		t.Logf("tracerpt output: %s", out)
		t.Fatalf("tracerpt failed: %v", err)
	}

	// Postprocess both dumps into a `processed/` subdir: strip the
	// unregistered-source preamble from eventlog.txt, drop the standard
	// tracerpt columns from trace.csv, and reduce each to
	// time<TAB>level<TAB>message so the two streams diff cleanly.
	// Raw files stay untouched.
	processedDir := filepath.Join(e2eDir, "processed")
	test.That(t, os.MkdirAll(processedDir, 0o755), test.ShouldBeNil)
	processedEventlog := filepath.Join(processedDir, "eventlog.tsv")
	processedTrace := filepath.Join(processedDir, "trace.tsv")

	if err := processEventlog(eventlogPath, processedEventlog); err != nil {
		t.Logf("processEventlog: %v", err)
	}
	if err := processTrace(tracerptOut, processedTrace); err != nil {
		t.Logf("processTrace: %v", err)
	}

	// Compare the two processed streams. Every zapcore.Entry fans out
	// to both appenders, so after stripping each format's wrapper the
	// two TSVs should match line-for-line (modulo level case).
	eventlogLines, err := parseProcessedLog(processedEventlog)
	test.That(t, err, test.ShouldBeNil)
	traceLines, err := parseProcessedLog(processedTrace)
	test.That(t, err, test.ShouldBeNil)
	compareLogs(t, eventlogLines, traceLines)

	t.Logf("Test output:        %s", e2eDir)
	t.Logf("Test output (unix): %s", filepath.ToSlash(e2eDir))
	t.Logf("window:             %s -> %s", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
}

// eventlogPreamble matches the boilerplate Windows inserts when an event
// source isn't registered with a manifest. The actual log payload sits
// after this prefix, wrapped in single quotes by the writer's '+ msg +'
// concatenation in our dump-eventlog.ps1 generator.
var eventlogPreamble = "The following information is part of the event:'"

// processEventlog reads the raw Get-EventLog dump and writes a
// time\tLEVEL\tmessage TSV sorted chronologically by the embedded zap
// timestamp. The embedded payload is the TSV produced by getMessage in
// logging/windows_event_logger.go: time\tLEVEL\tlogger\tcaller\t
// message[\tfieldsJSON]. We take fields 0, 1, and 4.
//
// Sorting is necessary because Get-EventLog's TimeGenerated has
// 1-second precision, so Sort-Object can't break ties at millisecond
// granularity in the raw dump. The embedded zap timestamp is fixed-
// width UTC RFC3339 (DefaultTimeFormatStr), so lexicographic sort =
// chronological sort.
func processEventlog(in, out string) error {
	inFile, err := os.Open(in)
	if err != nil {
		return err
	}
	defer inFile.Close()

	var rows [][3]string
	sc := bufio.NewScanner(inFile)
	// Some log lines (network-check results) are large; raise the line
	// cap from bufio's 64 KB default.
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		idx := strings.Index(line, eventlogPreamble)
		if idx < 0 {
			continue
		}
		body := strings.TrimSuffix(line[idx+len(eventlogPreamble):], "'")
		parts := strings.Split(body, "\t")
		if len(parts) < 5 {
			continue
		}
		rows = append(rows, [3]string{parts[0], parts[1], parts[4]})
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return writeSortedRows(out, rows)
}

// traceQuoted finds the quoted user-data fields in a tracerpt CSV row.
// tracerpt doesn't escape inner quotes for the fields JSON column, so
// matching after that field is unreliable — but the first five fields
// (time, level, logger, caller, message) don't contain quotes in
// practice, and we only need three of them.
var traceQuoted = regexp.MustCompile(`"([^"]*)"`)

// processTrace reads the raw tracerpt CSV and writes a
// time\tlevel\tmessage TSV sorted chronologically by the embedded zap
// timestamp. Skips non-LogEntry rows (the EventTrace header rows
// tracerpt emits at the top of the file). The first five quoted
// strings in a LogEntry row are the first five user-data fields — we
// take indices 0, 1, and 4.
//
// Sorting matters because ETW buffer interleaving across CPUs/threads
// can deliver events slightly out of timestamp order on busy systems,
// even though it usually looks sorted on a quiet run.
func processTrace(in, out string) error {
	inFile, err := os.Open(in)
	if err != nil {
		return err
	}
	defer inFile.Close()

	var rows [][3]string
	br := bufio.NewReader(inFile)
	for {
		line, readErr := br.ReadString('\n')
		if len(line) > 0 {
			row := strings.TrimRight(line, "\r\n")
			first, _, ok := strings.Cut(row, ",")
			if ok && strings.TrimSpace(first) == "LogEntry" {
				matches := traceQuoted.FindAllStringSubmatch(row, 5)
				if len(matches) >= 5 {
					rows = append(rows, [3]string{matches[0][1], matches[1][1], matches[4][1]})
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return writeSortedRows(out, rows)
}

// writeSortedRows sorts rows chronologically by the timestamp in
// column 0 (relying on RFC3339 fixed-width UTC being lex-sortable) and
// writes them to out as tab-separated lines.
func writeSortedRows(out string, rows [][3]string) error {
	sort.Slice(rows, func(i, j int) bool {
		// sort by log message if timestamp is the same
		if rows[i][0] == rows[j][0] {
			return rows[i][2] < rows[j][2]
		}
		return rows[i][0] < rows[j][0]
	})
	outFile, err := os.Create(out)
	if err != nil {
		return err
	}
	defer outFile.Close()
	w := bufio.NewWriter(outFile)
	defer w.Flush()
	for _, r := range rows {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", r[0], r[1], r[2]); err != nil {
			return err
		}
	}
	return nil
}

// logLine is a single time<TAB>level<TAB>message row parsed from a
// processed TSV. level case is preserved (eventlog emits UPPERCASE,
// trace emits lowercase) so comparisons can normalize on read.
type logLine struct {
	time, level, message string
}

// parseProcessedLog reads a time<TAB>level<TAB>message TSV. Rows with
// fewer than 3 fields are skipped silently.
func parseProcessedLog(path string) ([]logLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []logLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 3)
		if len(parts) < 3 {
			continue
		}
		lines = append(lines, logLine{time: parts[0], level: parts[1], message: parts[2]})
	}
	return lines, sc.Err()
}

// maxComparisonDiffs caps how many mismatch lines we surface via
// t.Errorf before collapsing the rest into a single overflow message.
// Prevents a totally broken run from flooding the test output.
const maxComparisonDiffs = 20

// compareLogs asserts that the two processed streams contain the same
// (time, message) pairs and that level matches case-insensitively.
//
// Both sides are re-sorted here by (time, message) so events with
// identical timestamps line up regardless of intra-second emission
// order across the two streams.
//
// Discrepancies fire t.Errorf (not t.Fatal) so all mismatches surface
// in one run and the artifact paths still print at the end.
func compareLogs(t *testing.T, eventlog, trace []logLine) {
	t.Helper()
	less := func(s []logLine) func(i, j int) bool {
		return func(i, j int) bool {
			if s[i].time != s[j].time {
				return s[i].time < s[j].time
			}
			return s[i].message < s[j].message
		}
	}
	sort.Slice(eventlog, less(eventlog))
	sort.Slice(trace, less(trace))

	diffs := 0
	report := func(format string, args ...any) {
		if diffs < maxComparisonDiffs {
			t.Errorf(format, args...)
		}
		diffs++
	}

	i, j := 0, 0
	for i < len(eventlog) && j < len(trace) {
		e, tr := eventlog[i], trace[j]
		if e.time == tr.time && e.message == tr.message {
			if !strings.EqualFold(e.level, tr.level) {
				report("level mismatch @ %s msg=%q: eventlog=%q trace=%q",
					e.time, e.message, e.level, tr.level)
			}
			i++
			j++
			continue
		}
		// Heads diverge — advance the side that's "earlier" so the
		// other side gets a chance to catch up. Whichever side falls
		// behind is the one missing this entry.
		eKey := e.time + "\x00" + e.message
		trKey := tr.time + "\x00" + tr.message
		if eKey < trKey {
			report("only in eventlog: %s %s %q", e.time, e.level, e.message)
			i++
		} else {
			report("only in trace:    %s %s %q", tr.time, tr.level, tr.message)
			j++
		}
	}
	for ; i < len(eventlog); i++ {
		report("only in eventlog: %s %s %q", eventlog[i].time, eventlog[i].level, eventlog[i].message)
	}
	for ; j < len(trace); j++ {
		report("only in trace:    %s %s %q", trace[j].time, trace[j].level, trace[j].message)
	}

	if diffs > maxComparisonDiffs {
		t.Errorf("... and %d more mismatches", diffs-maxComparisonDiffs)
	}
	if diffs == 0 {
		t.Logf("comparison: %d lines matched in both streams", len(eventlog))
	} else {
		t.Errorf("comparison: %d total mismatches (eventlog=%d trace=%d)",
			diffs, len(eventlog), len(trace))
	}
}

// isElevated reports whether the current process is a member of the
// local Administrators group. ETW session start in entrypoint.go
// requires this; without it, logman create silently fails and no
// .etl file is produced.
func isElevated() bool {
	var sid *windows.SID
	if err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	); err != nil {
		return false
	}
	defer windows.FreeSid(sid)
	member, err := windows.Token(0).IsMember(sid)
	return err == nil && member
}
