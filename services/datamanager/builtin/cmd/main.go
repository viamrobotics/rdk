// package main prints a disk summary of the builtin data manager's capture directory
// or additional sync paths
// It exists purely as a convenience utilty for viam developers & solutions engineers.
// Delete it if it becomes onerous to maintain.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/services/datamanager/builtin"
)

func main() {
	if len(os.Args) != 2 {
		//nolint:forbidigo
		fmt.Printf("usage: %s path\n", os.Args[0])
		os.Exit(1)
	}
	summary := builtin.DiskSummary(context.Background(), os.Args[1])
	fi := deriveFormatInfo(summary)
	for _, ds := range summary {
		//nolint:forbidigo
		fmt.Println(format(ds, fi))
	}
}

type formatInfo struct {
	maxFilePathLen   int
	maxFileSizeLen   int
	maxFileCounteLen int
	maxErrLen        int
	maxDurationLen   int
}

func deriveFormatInfo(summary []builtin.DirSummary) formatInfo {
	var (
		maxFilePathLen   int
		maxFileSizeLen   int
		maxFileCounteLen int
		maxErrLen        int
		maxDurationLen   int
	)

	for _, s := range summary {
		maxFilePathLen = max(maxFilePathLen, len(s.Path))
		maxFileSizeLen = max(maxFileSizeLen, len(data.FormatBytesI64(s.FileSize)))
		maxFileCounteLen = max(maxFileCounteLen, len(fmt.Sprintf("%d", s.FileCount)))
		if s.Err != nil {
			maxErrLen = max(maxErrLen, len(s.Err.Error()))
		}
		if s.DataTimeRange != nil {
			maxDurationLen = max(maxDurationLen, len(duration(s.DataTimeRange)))
		}
	}

	return formatInfo{
		maxFilePathLen,
		maxFileSizeLen,
		maxFileCounteLen,
		maxErrLen,
		maxDurationLen,
	}
}

func format(ds builtin.DirSummary, fi formatInfo) string {
	paddedPath := ds.Path + strings.Repeat(" ", 1+fi.maxFilePathLen-len(ds.Path)) + "| "
	countStr := fmt.Sprintf("%d", ds.FileCount)
	paddedCount := strings.Repeat(" ", fi.maxFileCounteLen-len(countStr)) + countStr + " |"
	sizeStr := data.FormatBytesI64(ds.FileSize)
	paddedSize := strings.Repeat(" ", fi.maxFileSizeLen-len(sizeStr)) + sizeStr + " |"
	dataTimeRangeStr := formatDataTimeRange(ds.DataTimeRange, fi)
	if dataTimeRangeStr != "" {
		dataTimeRangeStr = fmt.Sprintf(" %s", dataTimeRangeStr)
	}
	var errStr string
	if ds.Err != nil {
		errStr = fmt.Sprintf(" error: %s", ds.Err)
	}

	return fmt.Sprintf("%sfile_count: %s file_size: %s%s%s", paddedPath, paddedCount, paddedSize, dataTimeRangeStr, errStr)
}

func formatDataTimeRange(dtr *builtin.DataTimeRange, fi formatInfo) string {
	if dtr == nil {
		return ""
	}
	paddedDuration := strings.Repeat(" ", fi.maxDurationLen-len(duration(dtr))) + duration(dtr)
	return fmt.Sprintf("duration: %s, start: %s, end: %s", paddedDuration, dtr.Start, dtr.End)
}

func duration(dtr *builtin.DataTimeRange) string {
	if dtr == nil {
		return ""
	}
	return dtr.End.Sub(dtr.Start).String()
}
