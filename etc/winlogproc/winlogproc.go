// Package winlogproc converts the raw eventlog.txt and trace.csv
// dumps produced by the Windows logging e2e test (or any equivalent
// Get-EventLog / tracerpt pair) into compact time<TAB>level<TAB>message
// TSVs sorted chronologically by the embedded zap timestamp.
//
// The two dumps come from completely different Windows logging systems
// but capture the same upstream zapcore entries — these processors
// strip the per-system framing so the underlying log content can be
// compared directly.
//
// winlogproc is only called by developers and an e2e test that is
// normally gated, so it carries per-line nolint directives for file
// I/O on user-supplied paths.
package winlogproc

import (
	"braces.dev/errtrace"
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// traceTimeFormat matches logging.DefaultTimeFormatStr — the format
// the ETW appender uses for the "time" field. Duplicated rather than
// imported so winlogproc doesn't depend on rdk-logging.
const traceTimeFormat = "2006-01-02T15:04:05.000Z0700"

// eventlogPreamble is the boilerplate Windows inserts when an event
// source isn't registered with a manifest. The actual log payload
// follows, single-quote-wrapped by the dump script.
const eventlogPreamble = "The following information is part of the event:'"

// Eventlog reads a raw Get-EventLog dump and writes a
// time<TAB>LEVEL<TAB>message TSV sorted chronologically by the
// embedded zap timestamp.
//
// The embedded payload is the TSV produced by getMessage in
// logging/windows_event_logger.go: time\tLEVEL\tlogger\tcaller\t
// message[\tfieldsJSON]. We take fields 0, 1, and 4.
//
// Sorting is necessary because Get-EventLog's TimeGenerated has
// 1-second precision, so Sort-Object can't break ties at millisecond
// granularity in the raw dump. The embedded zap timestamp is
// fixed-width UTC RFC3339, so lexicographic sort == chronological
// sort.
func Eventlog(in, out string) error {
	// This utility reads log files from a variable path, so there's no way around file reads
	//nolint:gosec
	inFile, err := os.Open(in)
	if err != nil {
		return errtrace.Wrap(err)
	}
	defer inFile.Close() //nolint:errcheck // read-only file.

	var rows [][4]string
	sc := bufio.NewScanner(inFile)
	// Some log lines (network-check results) are large; raise the line
	// cap from bufio's 64 KB default.
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		_, body, ok := strings.Cut(sc.Text(), eventlogPreamble)
		if !ok {
			continue
		}
		parts := strings.Split(strings.TrimSuffix(body, "'"), "\t")
		if len(parts) < 5 {
			continue
		}
		rows = append(rows, [4]string{parts[0], parts[1], parts[3], parts[4]})
	}
	if err := sc.Err(); err != nil {
		return errtrace.Wrap(err)
	}
	return errtrace.Wrap(writeSortedRows(out, rows))
}

// traceEvent matches the shape tracerpt -of XML emits per ETW event.
// We only need the named Data children that carry our TraceLogging
// fields; non-LogEntry events (e.g. tracerpt's own EventTrace headers)
// have completely different Data names and get filtered out by the
// "must have a time field" guard in Trace.
type traceEvent struct {
	EventData struct {
		Data []struct {
			Name  string `xml:"Name,attr"`
			Value string `xml:",chardata"`
		} `xml:"Data"`
	} `xml:"EventData"`
}

// Trace reads a raw tracerpt XML dump and writes a
// time<TAB>level<TAB>caller<TAB>message TSV sorted chronologically by
// the embedded zap timestamp. Skips events without a "time" Data field
// (tracerpt's own EventTrace header events).
//
// after and before optionally bound the window. Zero values mean
// unbounded. tracerpt itself has no time filter, so we apply it here.
//
// Sorting matters because ETW buffer interleaving across CPUs/threads
// can deliver events slightly out of timestamp order on busy systems.
func Trace(in, out string, after, before time.Time) error {
	// This utility reads log files from a variable path, so there's no way around file reads
	//nolint:gosec
	inFile, err := os.Open(in)
	if err != nil {
		return errtrace.Wrap(err)
	}
	defer inFile.Close() //nolint:errcheck // read-only file.

	var rows [][4]string
	dec := xml.NewDecoder(inFile)
	for {
		tok, tokErr := dec.Token()
		if errors.Is(tokErr, io.EOF) {
			break
		}
		if tokErr != nil {
			return errtrace.Wrap(tokErr)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Event" {
			continue
		}
		var ev traceEvent
		if err := dec.DecodeElement(&ev, &se); err != nil {
			return errtrace.Wrap(err)
		}
		var t, lvl, cal, msg string
		for _, d := range ev.EventData.Data {
			switch d.Name {
			case "time":
				t = d.Value
			case "level":
				lvl = d.Value
			case "caller":
				cal = d.Value
			case "message":
				msg = d.Value
			}
		}
		if t == "" {
			continue
		}
		if !after.IsZero() || !before.IsZero() {
			ts, err := time.Parse(traceTimeFormat, t)
			if err == nil {
				if !after.IsZero() && ts.Before(after) {
					continue
				}
				if !before.IsZero() && ts.After(before) {
					continue
				}
			}
		}
		rows = append(rows, [4]string{t, lvl, cal, msg})
	}
	return errtrace.Wrap(writeSortedRows(out, rows))
}

// MaxTimestampInTSV scans a processed time<TAB>level<TAB>caller<TAB>
// message TSV and returns the latest parseable timestamp in column 0.
// Returns zero time if the file is empty or no rows parse.
func MaxTimestampInTSV(path string) (time.Time, error) {
	// This utility reads log files from a variable path, so there's no way around file reads
	//nolint:gosec
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, errtrace.Wrap(err)
	}
	defer f.Close() //nolint:errcheck // read-only file.
	var maxTime time.Time
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		first, _, ok := strings.Cut(sc.Text(), "\t")
		if !ok {
			continue
		}
		ts, err := time.Parse(traceTimeFormat, first)
		if err != nil {
			continue
		}
		if ts.After(maxTime) {
			maxTime = ts
		}
	}
	return maxTime, errtrace.Wrap(sc.Err())
}

// FilterProcessedTSV rewrites a processed time<TAB>level<TAB>caller<TAB>
// message TSV in place, keeping only rows whose timestamp is within
// [after, before]. Zero values mean unbounded on that side. Rows whose
// timestamp can't be parsed are kept (better than dropping mystery data).
func FilterProcessedTSV(path string, after, before time.Time) error {
	// This utility reads log files from a variable path, so there's no way around file reads
	//nolint:gosec
	in, err := os.ReadFile(path)
	if err != nil {
		return errtrace.Wrap(err)
	}
	var kept []string
	for _, line := range strings.Split(string(in), "\n") {
		if line == "" {
			continue
		}
		first, _, ok := strings.Cut(line, "\t")
		if !ok {
			kept = append(kept, line)
			continue
		}
		ts, err := time.Parse(traceTimeFormat, first)
		if err == nil {
			if !after.IsZero() && ts.Before(after) {
				continue
			}
			if !before.IsZero() && ts.After(before) {
				continue
			}
		}
		kept = append(kept, line)
	}
	//nolint:gosec // CLI-supplied path; rewrites the file we just read.
	out, err := os.Create(path)
	if err != nil {
		return errtrace.Wrap(err)
	}
	defer out.Close() //nolint:errcheck // best-effort; write errors surface via WriteString below.
	w := bufio.NewWriter(out)
	defer w.Flush() //nolint:errcheck // best-effort flush on the deferred path.
	for _, line := range kept {
		if _, err := w.WriteString(line + "\n"); err != nil {
			return errtrace.Wrap(err)
		}
	}
	return nil
}

// writeSortedRows sorts rows chronologically by column 0 (relying on
// fixed-width RFC3339 UTC being lex-sortable) and writes them to out
// as tab-separated lines: time<TAB>level<TAB>caller<TAB>message.
func writeSortedRows(out string, rows [][4]string) error {
	sort.Slice(rows, func(i, j int) bool {
		// Break timestamp ties on message text so two streams of the
		// same events sort to identical order regardless of which side
		// happened to win the in-stream race.
		if rows[i][0] == rows[j][0] {
			return rows[i][3] < rows[j][3]
		}
		return rows[i][0] < rows[j][0]
	})
	//nolint:gosec // CLI-supplied output path.
	outFile, err := os.Create(out)
	if err != nil {
		return errtrace.Wrap(err)
	}
	defer outFile.Close() //nolint:errcheck
	w := bufio.NewWriter(outFile)
	defer w.Flush() //nolint:errcheck
	for _, r := range rows {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r[0], r[1], r[2], r[3]); err != nil {
			return errtrace.Wrap(err)
		}
	}
	return nil
}
