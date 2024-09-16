package builtin

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.viam.com/rdk/data"
)

// DirSummary represents a summary of the files in that directory including file counts,
// total size of files in that directory and the time range of data capture files
// (both .capture and .prog) in that directory.
// File and directory errors are accumulated on Err.
type DirSummary struct {
	Path          string
	FileSize      int64
	FileCount     int64
	Err           error
	DataTimeRange *DataTimeRange
}

// DataTimeRange represents a time range from Start to End.
type DataTimeRange struct {
	Start time.Time
	End   time.Time
}

// DiskSummary summarizes the provided sync path and all sub directories.
// The rootPath can either be the viam capture dir which data capture collectors
// write to or an additional sync path that contains arbitrary files.
// It will return a slice of DirSummary structs.
// Directories with no filesare ignored.
func DiskSummary(ctx context.Context, rootPath string) []DirSummary {
	if ctx.Err() != nil {
		return nil
	}
	// read the dir
	children, err := os.ReadDir(rootPath)
	if err != nil {
		return []DirSummary{{Path: rootPath, Err: err}}
	}
	// For each dirElement, sum up the size of the files in this directory
	// call self func on all directories and add result to return
	var (
		fileSize      int64
		fileCount     int64
		summary       []DirSummary
		dirPaths      []string
		dataTimeRange *DataTimeRange
		rootErr       error
	)
	for _, child := range children {
		if ctx.Err() != nil {
			return summary
		}
		path := filepath.Join(rootPath, child.Name())
		if child.IsDir() {
			// aggrigate all root's children
			dirPaths = append(dirPaths, path)
		} else {
			dataTimeRange = parseTimeRange(child.Name(), dataTimeRange)
			fileCount++
			info, err := child.Info()
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				// aggregate all errors encountered on files in this directory
				// TODO: Test ErrPermission actually happens if you don't have permissions
				rootErr = errors.Join(rootErr, err)
				continue
			}
			// sum up the size of all files in the root
			fileSize += info.Size()
		}
	}

	// if there were files in this directory, record the size
	if fileCount != 0 || dataTimeRange != nil {
		summary = append(summary, DirSummary{
			Path:          rootPath,
			FileSize:      fileSize,
			FileCount:     fileCount,
			Err:           rootErr,
			DataTimeRange: dataTimeRange,
		})
	}
	// do the same for all children
	for _, dirPath := range dirPaths {
		if ctx.Err() != nil {
			return summary
		}
		summary = append(summary, DiskSummary(ctx, dirPath)...)
	}

	return summary
}

func parseTimeRange(name string, dataTimeRange *DataTimeRange) *DataTimeRange {
	tPtr := parseTime(name)
	if tPtr == nil {
		// If we can't parse out the time from the file name, return the existing *DataTimeRange
		return dataTimeRange
	}

	t := *tPtr
	if dataTimeRange == nil {
		// if we did parse a datetime and don't have a *DataTimeRange yet,
		// the datetime range is the newly parsed time
		return &DataTimeRange{Start: t, End: t}
	}

	if t.Before(dataTimeRange.Start) {
		// if the parsed datetime is before the start of the range,
		// the parsed datetime is the new start of the time range
		dataTimeRange.Start = t
	}

	if t.After(dataTimeRange.End) {
		// if the parsed datetime is after the end of the range,
		// the parsed datetime is the new end of the time range
		dataTimeRange.End = t
	}
	return dataTimeRange
}

func parseTime(name string) *time.Time {
	if strings.HasSuffix(name, data.CompletedCaptureFileExt) {
		// this needs to undo data.CaptureFilePathWithReplacedReservedChars to get back a parsable RFC3339Nano date
		t, err := time.Parse(time.RFC3339Nano, strings.ReplaceAll(strings.TrimSuffix(name, data.CompletedCaptureFileExt), "_", ":"))
		if err != nil {
			// failed to parse
			return nil
		}
		return &t
	}

	if strings.HasSuffix(name, data.InProgressCaptureFileExt) {
		// this needs to undo data.CaptureFilePathWithReplacedReservedChars to get back a parsable RFC3339Nano date
		t, err := time.Parse(time.RFC3339Nano, strings.ReplaceAll(strings.TrimSuffix(name, data.InProgressCaptureFileExt), "_", ":"))
		if err != nil {
			// failed to parse
			return nil
		}
		return &t
	}
	return nil
}
