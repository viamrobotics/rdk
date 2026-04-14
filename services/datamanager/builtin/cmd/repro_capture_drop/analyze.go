// Package main analyzes capture files from the counter sensor module for
// dropped readings during collector reinitialization.
//
// The counter sensor returns {"count": N} where N increments by 1 on each
// Readings call. This tool reads all capture files, extracts the counter
// values in file order, and reports any gaps in the sequence. A gap means
// data was polled from the sensor but never written to disk.
//
// Usage:
//
//	go run analyze.go <capture_dir>
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"go.viam.com/rdk/data"
)

type fileEntry struct {
	name   string
	counts []int64
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <capture_dir>\n", os.Args[0])
		os.Exit(1)
	}
	dir := os.Args[1]

	var files []fileEntry

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".capture" && ext != ".prog" {
			return nil
		}
		readings, err := data.SensorDataFromCaptureFilePath(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
			return nil
		}
		fe := fileEntry{name: filepath.Base(path)}
		for _, r := range readings {
			s := r.GetStruct()
			if s == nil {
				continue
			}
			readingsField := s.GetFields()["readings"]
			if readingsField == nil {
				continue
			}
			inner := readingsField.GetStructValue()
			if inner == nil {
				continue
			}
			countField := inner.GetFields()["count"]
			if countField == nil {
				continue
			}
			fe.counts = append(fe.counts, int64(countField.GetNumberValue()))
		}
		files = append(files, fe)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error walking directory: %v\n", err)
		os.Exit(1)
	}

	// Sort files by their first counter value so pre-reconfig comes before post-reconfig.
	sort.Slice(files, func(i, j int) bool {
		if len(files[i].counts) == 0 {
			return true
		}
		if len(files[j].counts) == 0 {
			return false
		}
		return files[i].counts[0] < files[j].counts[0]
	})

	// Concatenate counts in file order without sorting individual readings.
	var allCounts []int64
	fmt.Println("Files:")
	for _, f := range files {
		if len(f.counts) > 0 {
			fmt.Printf("  %s: %d readings, count range [%d -> %d]\n",
				f.name, len(f.counts), f.counts[0], f.counts[len(f.counts)-1])
		} else {
			fmt.Printf("  %s: 0 readings\n", f.name)
		}
		allCounts = append(allCounts, f.counts...)
	}

	if len(allCounts) == 0 {
		fmt.Println("No readings found.")
		return
	}

	// Walk the sequence linearly and report anomalies.
	fmt.Println("\nSequence analysis:")
	type gap struct {
		after   int64
		before  int64
		missing int
	}
	var gaps []gap
	var duplicates int
	var outOfOrder int

	for i := 1; i < len(allCounts); i++ {
		diff := allCounts[i] - allCounts[i-1]
		switch {
		case diff > 1:
			gaps = append(gaps, gap{
				after:   allCounts[i-1],
				before:  allCounts[i],
				missing: int(diff - 1),
			})
		case diff == 0:
			duplicates++
			fmt.Printf("  WARNING: duplicate count %d at position %d\n", allCounts[i], i)
		case diff < 0:
			outOfOrder++
			fmt.Printf("  WARNING: out-of-order at position %d: %d followed by %d\n",
				i, allCounts[i-1], allCounts[i])
		}
	}

	totalMissing := 0
	if len(gaps) > 0 {
		fmt.Println("  GAPS DETECTED (data polled from sensor but never written to disk):")
		for _, g := range gaps {
			totalMissing += g.missing
			if g.missing <= 20 {
				fmt.Printf("    after count %d, before count %d: missing %d values (",
					g.after, g.before, g.missing)
				for j := 0; j < g.missing; j++ {
					if j > 0 {
						fmt.Print(", ")
					}
					fmt.Printf("%d", g.after+int64(j)+1)
				}
				fmt.Println(")")
			} else {
				fmt.Printf("    after count %d, before count %d: missing %d values (%d..%d)\n",
					g.after, g.before, g.missing, g.after+1, g.before-1)
			}
		}
	} else {
		fmt.Println("  No gaps found - counter sequence is contiguous.")
	}

	expectedCount := allCounts[len(allCounts)-1] - allCounts[0] + 1
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Counter range:   %d to %d\n", allCounts[0], allCounts[len(allCounts)-1])
	fmt.Printf("  Expected:        %d readings\n", expectedCount)
	fmt.Printf("  Actual:          %d readings\n", len(allCounts))
	fmt.Printf("  Dropped:         %d readings across %d gap(s)\n", totalMissing, len(gaps))
	if duplicates > 0 {
		fmt.Printf("  Duplicates:      %d\n", duplicates)
	}
	if outOfOrder > 0 {
		fmt.Printf("  Out-of-order:    %d\n", outOfOrder)
	}
}
