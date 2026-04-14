// Package main analyzes capture files for data loss during collector reinitialization.
// It reads all .capture and .prog files in the given directory, counts readings,
// and reports gaps between files that indicate dropped data.
//
// Usage:
//
//	go run analyze.go <capture_dir> [expected_hz]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"go.viam.com/rdk/data"
)

type captureFileInfo struct {
	name    string
	count   int
	firstTS time.Time
	lastTS  time.Time
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <capture_dir> [expected_hz]\n", os.Args[0])
		os.Exit(1)
	}
	dir := os.Args[1]
	hz := 100.0
	if len(os.Args) >= 3 {
		if v, err := strconv.ParseFloat(os.Args[2], 64); err == nil {
			hz = v
		}
	}

	var files []captureFileInfo
	var total int

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
		fi := captureFileInfo{name: filepath.Base(path), count: len(readings)}
		if len(readings) > 0 {
			fi.firstTS = readings[0].GetMetadata().GetTimeRequested().AsTime()
			fi.lastTS = readings[len(readings)-1].GetMetadata().GetTimeRequested().AsTime()
		}
		files = append(files, fi)
		total += len(readings)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error walking directory: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No capture files found.")
		return
	}

	sort.Slice(files, func(i, j int) bool { return files[i].firstTS.Before(files[j].firstTS) })

	fmt.Println("Files:")
	for _, f := range files {
		fmt.Printf("  %s: %d readings [%s -> %s]\n",
			f.name, f.count,
			f.firstTS.Format("15:04:05.000"),
			f.lastTS.Format("15:04:05.000"))
	}

	fmt.Println("\nGaps:")
	var totalGap time.Duration
	var potentialLoss int
	for i := 1; i < len(files); i++ {
		gap := files[i].firstTS.Sub(files[i-1].lastTS)
		missed := int(gap.Seconds() * hz)
		fmt.Printf("  Between file %d and %d: %v", i, i+1, gap)
		if gap > time.Duration(float64(time.Second)/hz)*2 {
			fmt.Printf("  *** ~%d readings missing ***", missed)
			potentialLoss += missed
			totalGap += gap
		}
		fmt.Println()
	}

	totalDuration := files[len(files)-1].lastTS.Sub(files[0].firstTS)
	expected := int(totalDuration.Seconds() * hz)

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Total duration:       %v\n", totalDuration)
	fmt.Printf("  Total readings:       %d\n", total)
	fmt.Printf("  Expected at %.0fHz:    ~%d\n", hz, expected)
	fmt.Printf("  Difference:           %d\n", expected-total)
	if potentialLoss > 0 {
		fmt.Printf("  Gap-based loss est:   ~%d readings across %v of gaps\n", potentialLoss, totalGap)
	}
}
