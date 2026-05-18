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
	"encoding/xml"
	"fmt"
	"io"
	"os"
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
// the embedded zap timestamp. Skips events whose RenderingInfo
// EventName isn't "LogEntry" (the EventTrace header events tracerpt
// emits, and anything else that may share the .etl).
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
	dec := xml.NewDecoder(inFile)
	for {
		tok, tokErr := dec.Token()
		if tokErr == io.EOF {
			break
		}
		if tokErr != nil {
			return tokErr
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Event" {
			continue
		}
		var ev traceEvent
		if err := dec.DecodeElement(&ev, &se); err != nil {
			return err
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
		rows = append(rows, [4]string{t, lvl, cal, msg})
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
