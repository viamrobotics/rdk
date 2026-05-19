//go:build windows && etw_e2e

package server_test

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	"go.viam.com/rdk/etc/winlogproc"
)

const (
	// runDuration is how long viam-server runs before the test sends
	// Ctrl+Break. Bump this to capture more log volume.
	runDuration = 15 * time.Second

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

	// Collect: flush + tracerpt + Get-EventLog + process, all in one
	// call against the test's per-run VIAM_HOME. After is set to
	// startTime so unrelated viam-server runs on this machine don't
	// bleed into the eventlog dump.
	collectDir, err := winlogproc.Collect(winlogproc.CollectOpts{
		ETLDir: filepath.Join(e2eDir, "logs"),
		OutDir: e2eDir,
		After:  startTime,
		Before: endTime,
	})
	test.That(t, err, test.ShouldBeNil)
	processedEventlog := filepath.Join(collectDir, "processed", "eventlog.tsv")
	processedTrace := filepath.Join(collectDir, "processed", "trace.tsv")

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

// logLine is a single time<TAB>level<TAB>caller<TAB>message row parsed
// from a processed TSV. level case is preserved (eventlog emits
// UPPERCASE, trace emits lowercase) so comparisons can normalize on
// read. message uses SplitN with N=4 so embedded tabs inside the
// message field (e.g. module logs with smuggled-caller payloads)
// survive into the parsed message string instead of getting split out.
type logLine struct {
	time, level, caller, message string
}

// parseProcessedLog reads a time<TAB>level<TAB>caller<TAB>message TSV.
// Rows with fewer than 4 fields are skipped silently.
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
		parts := strings.SplitN(sc.Text(), "\t", 4)
		if len(parts) < 4 {
			continue
		}
		lines = append(lines, logLine{time: parts[0], level: parts[1], caller: parts[2], message: parts[3]})
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
			if e.caller != tr.caller {
				report("caller mismatch @ %s msg=%q: eventlog=%q trace=%q",
					e.time, e.message, e.caller, tr.caller)
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
			report("only in eventlog: %s %s caller=%q msg=%q", e.time, e.level, e.caller, e.message)
			i++
		} else {
			report("only in trace:    %s %s caller=%q msg=%q", tr.time, tr.level, tr.caller, tr.message)
			j++
		}
	}
	for ; i < len(eventlog); i++ {
		report("only in eventlog: %s %s caller=%q msg=%q", eventlog[i].time, eventlog[i].level, eventlog[i].caller, eventlog[i].message)
	}
	for ; j < len(trace); j++ {
		report("only in trace:    %s %s caller=%q msg=%q", trace[j].time, trace[j].level, trace[j].caller, trace[j].message)
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
