// Package winlogproc converts the raw eventlog.txt and trace.csv
// dumps produced by the Windows logging e2e test (or any equivalent
// Get-EventLog / tracerpt pair) into compact time<TAB>level<TAB>message
// TSVs sorted chronologically by the embedded zap timestamp.
//
// The two dumps come from completely different Windows logging systems
// but capture the same upstream zapcore entries — these processors
// strip the per-system framing so the underlying log content can be
// compared directly.
package winlogproc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

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
	inFile, err := os.Open(in)
	if err != nil {
		return err
	}
	defer inFile.Close()

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

// Trace reads a raw tracerpt CSV and writes a time<TAB>level<TAB>message
// TSV sorted chronologically by the embedded zap timestamp. Skips
// non-LogEntry rows (the EventTrace header rows tracerpt emits at the
// top of the file). The first five quoted strings in a LogEntry row
// are the first five user-data fields — we take indices 0, 1, and 4.
//
// Sorting matters because ETW buffer interleaving across CPUs/threads
// can deliver events slightly out of timestamp order on busy systems.
func Trace(in, out string) error {
	inFile, err := os.Open(in)
	if err != nil {
		return err
	}
	defer inFile.Close()

	var rows [][4]string
	br := bufio.NewReader(inFile)
	for {
		line, readErr := br.ReadString('\n')
		if len(line) > 0 {
			row := strings.TrimRight(line, "\r\n")
			first, _, ok := strings.Cut(row, ",")
			if ok && strings.TrimSpace(first) == "LogEntry" {
				matches := traceQuoted.FindAllStringSubmatch(row, 5)
				if len(matches) >= 5 {
					rows = append(rows, [4]string{matches[0][1], matches[1][1], matches[3][1], matches[4][1]})
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
	outFile, err := os.Create(out)
	if err != nil {
		return err
	}
	defer outFile.Close()
	w := bufio.NewWriter(outFile)
	defer w.Flush()
	for _, r := range rows {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r[0], r[1], r[2], r[3]); err != nil {
			return err
		}
	}
	return nil
}
