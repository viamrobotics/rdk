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
	"go.viam.com/rdk/utils"
)

// DirSummary represents a summary of the files in that directory including file counts,
// total size of files in that directory and the time range of the data in that directory.
// File and directory errors are accumulated on Err.
type DirSummary struct {
	Path      string
	FileSize  int64
	FileCount int64
	Err       error
	// DataTimeRange is the time range across all files in the directory. Capture files
	// (.capture/.prog) use the timestamp encoded in their name; arbitrary files use their
	// filesystem creation time.
	DataTimeRange *DataTimeRange
	// SyncableFileTimeRange is the time range of files that are complete and waiting to
	// sync: completed capture (.capture) files and arbitrary files. In-progress (.prog)
	// capture files are excluded since they are still being written.
	SyncableFileTimeRange *DataTimeRange
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
		fileSize              int64
		fileCount             int64
		summary               []DirSummary
		dirPaths              []string
		dataTimeRange         *DataTimeRange
		syncableFileTimeRange *DataTimeRange
		rootErr               error
	)
	for _, child := range children {
		if ctx.Err() != nil {
			return summary
		}
		path := filepath.Join(rootPath, child.Name())
		if child.IsDir() {
			// aggregate all root's children
			dirPaths = append(dirPaths, path)
			continue
		}

		name := child.Name()
		fileCount++
		info, err := child.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			rootErr = errors.Join(rootErr, err)
			continue
		}
		// sum up the size of all files in the root
		fileSize += info.Size()

		isCompletedCapture := strings.HasSuffix(name, data.CompletedCaptureFileExt)
		isInProgressCapture := strings.HasSuffix(name, data.InProgressCaptureFileExt)

		// Capture files (.capture/.prog) encode the capture time in their name.
		fileTime := parseTime(name)

		// Arbitrary file uploads, fall back to filesystem creation time (same timestamp
		// used for sync).
		if fileTime == nil && !isCompletedCapture && !isInProgressCapture {
			fileTimes, err := utils.GetFileTimes(path)
			if err != nil {
				rootErr = errors.Join(rootErr, err)
			} else {
				fileTime = &fileTimes.CreateTime
			}
		}

		dataTimeRange = extendTimeRange(dataTimeRange, fileTime)
		// Completed capture files and arbitrary files are fully written and waiting to
		// sync; in-progress (.prog) files are excluded since they're still being written.
		if !isInProgressCapture {
			syncableFileTimeRange = extendTimeRange(syncableFileTimeRange, fileTime)
		}
	}

	// if there were files in this directory, record the size
	if fileCount != 0 || dataTimeRange != nil {
		summary = append(summary, DirSummary{
			Path:                  rootPath,
			FileSize:              fileSize,
			FileCount:             fileCount,
			Err:                   rootErr,
			DataTimeRange:         dataTimeRange,
			SyncableFileTimeRange: syncableFileTimeRange,
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

// extendTimeRange grows dataTimeRange to include tPtr, returning the updated range.
func extendTimeRange(dataTimeRange *DataTimeRange, tPtr *time.Time) *DataTimeRange {
	if tPtr == nil {
		// If we don't have a time for this file, return the existing *DataTimeRange
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
