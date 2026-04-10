package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

// UploadChunkSize defines the size of the data included in each message of a FileUpload stream.
var (
	UploadChunkSize            = 64 * 1024
	errFileEmpty               = errors.New("file is empty (0 bytes)")
	errFileModifiedTooRecently = errors.New("file modified too recently")
)

// uploadArbitraryFile uploads files which were not writted by the builtin datamanager's data capture package.
// They are frequently files written by 3rd party programs such as images, videos, logs, written to
// the capture directory or a subdirectory or to additional sync paths (or their sub directories).
// Note: the bytes size returned is the size of the input file. It only returns a non 0 value in the success case.
// If bytesUploadingCounter is provided, it will be updated as each chunk is successfully uploaded.
func uploadArbitraryFile(
	ctx context.Context,
	f *os.File,
	conn cloudConn,
	tags, datasetIDs []string,
	fileLastModifiedMillis int,
	clock clock.Clock,
	logger logging.Logger,
	bytesUploadingCounter *atomic.Uint64,
) (uint64, error) {
	logger.Debugf("attempting to sync arbitrary file: %s", f.Name())
	path, err := filepath.Abs(f.Name())
	if err != nil {
		return 0, errors.Wrap(err, "failed to get absolute path")
	}

	// Only sync non-datacapture files that have not been modified in the last
	// fileLastModifiedMillis to avoid uploading files that are being
	// to written to.
	info, err := os.Stat(path)
	if err != nil {
		return 0, errors.Wrap(err, "stat failed")
	}
	if info.Size() == 0 {
		return 0, errFileEmpty
	}

	timeSinceMod := clock.Since(info.ModTime())
	if timeSinceMod < time.Duration(fileLastModifiedMillis)*time.Millisecond {
		return 0, errFileModifiedTooRecently
	}

	// We need to seek to the start of the file as if we have tried to upload this file
	// previously, the read offset of the file might not be at the beginning, which would
	// result in partial data loss when uploading to the cloud.
	// Fixes https://viam.atlassian.net/browse/DATA-3114
	pos, err := f.Seek(0, io.SeekStart)
	if err != nil {
		return 0, errors.Wrap(err, "error trying to Seek to beginning of file")
	}

	if pos != 0 {
		return 0, fmt.Errorf("error trying to seek to beginning of file %s: expected position 0, instead got to position %d", path, pos)
	}

	// Get file timestamps
	fileTimes, err := utils.GetFileTimes(path)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get file times")
	}

	logger.Debugf("datasync.FileUpload request started for arbitrary file: %s", path)
	stream, err := conn.client.FileUpload(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "error creating FileUpload client")
	}

	// Try to infer tags and dataset IDs from the filename query parameters.
	uploadTagsSet := goutils.NewStringSet(tags...)
	uploadDatasetIDsSet := goutils.NewStringSet(datasetIDs...)
	inferredTags, inferredDatasetIDs := inferTagsAndDatasetIDsFromPath(path)
	for _, t := range inferredTags {
		uploadTagsSet.Add(t)
	}
	for _, id := range inferredDatasetIDs {
		uploadDatasetIDsSet.Add(id)
	}
	logger.Debugf(
		"inferred upload metadata from path segments; file=%q tags=%v datasetIDs=%v",
		path,
		inferredTags,
		inferredDatasetIDs,
	)
	uploadFileExt := filepath.Ext(path)

	// Send metadata FileUploadRequest.
	logger.Debugf("datasync.FileUpload request sending metadata for arbitrary file: %s", path)
	if err := stream.Send(&v1.FileUploadRequest{
		UploadPacket: &v1.FileUploadRequest_Metadata{
			Metadata: &v1.UploadMetadata{
				PartId:         conn.partID,
				Type:           v1.DataType_DATA_TYPE_FILE,
				FileName:       path,
				FileExtension:  uploadFileExt,
				FileCreateTime: timestamppb.New(fileTimes.CreateTime),
				FileModifyTime: timestamppb.New(fileTimes.ModifyTime),
				Tags:           uploadTagsSet.ToList(),
				DatasetIds:     uploadDatasetIDsSet.ToList(),
			},
		},
	}); err != nil {
		return 0, errors.Wrap(err, "FileUpload failed sending metadata")
	}

	if err := sendFileUploadRequests(ctx, stream, f, path, logger, bytesUploadingCounter); err != nil {
		return 0, errors.Wrap(err, "FileUpload failed to sync")
	}

	logger.Debugf("datasync.FileUpload closing for arbitrary file: %s", path)
	if _, err = stream.CloseAndRecv(); err != nil {
		return 0, errors.Wrap(err, "FileUpload  CloseAndRecv failed")
	}
	return uint64(info.Size()), nil
}

func sendFileUploadRequests(
	ctx context.Context,
	stream v1.DataSyncService_FileUploadClient,
	f *os.File,
	path string,
	logger logging.Logger,
	bytesUploadingCounter *atomic.Uint64,
) error {
	// Loop until there is no more content to be read from file.
	i := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextFileUploadRequest(f)

		// EOF means we've completed successfully.
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return err
		}

		logger.Debugf("datasync.FileUpload sending chunk %d for file: %s", i, path)
		if err = stream.Send(uploadReq); err != nil {
			return err
		}

		// Update byte counter after successful chunk upload.
		if bytesUploadingCounter != nil {
			if fileContents := uploadReq.GetFileContents(); fileContents != nil {
				chunkSize := uint64(len(fileContents.Data))
				bytesUploadingCounter.Add(chunkSize)
			}
		}

		i++
	}
}

func getNextFileUploadRequest(f *os.File) (*v1.FileUploadRequest, error) {
	// Get the next file data reading from file, check for an error.
	next, err := readNextFileChunk(f)
	if err != nil {
		return nil, err
	}
	// Otherwise, return an UploadRequest and no error.
	return &v1.FileUploadRequest{
		UploadPacket: &v1.FileUploadRequest_FileContents{
			FileContents: next,
		},
	}, nil
}

func readNextFileChunk(f *os.File) (*v1.FileData, error) {
	byteArr := make([]byte, UploadChunkSize)
	numBytesRead, err := f.Read(byteArr)
	if err != nil {
		return nil, err
	}
	return &v1.FileData{Data: byteArr[:numBytesRead]}, nil
}

// inferTagsAndDatasetIDsFromPath infers tags and dataset IDs from the path of a file.
// Directory name convention: `.../tag=<tag>/tag=<tag>/dataset=<id>/dataset=<id>/<file>`
func inferTagsAndDatasetIDsFromPath(path string) (tags, datasetIDs []string) {
	dir := filepath.Dir(path)
	for {
		seg := strings.TrimSpace(filepath.Base(dir))
		if seg != "" && seg != "." && seg != string(filepath.Separator) {
			if v, ok := strings.CutPrefix(seg, "tag="); ok {
				v = strings.TrimSpace(v)
				if v != "" {
					tags = append(tags, v)
				}
			} else if v, ok := strings.CutPrefix(seg, "dataset="); ok {
				v = strings.TrimSpace(v)
				if v != "" {
					datasetIDs = append(datasetIDs, v)
				}
			}
		}

		// Go up one directory until we can no longer ascend.
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return tags, datasetIDs
}
