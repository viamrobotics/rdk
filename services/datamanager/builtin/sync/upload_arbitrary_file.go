package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"

	"go.viam.com/rdk/logging"
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
func uploadArbitraryFile(
	ctx context.Context,
	f *os.File,
	conn cloudConn,
	tags []string,
	fileLastModifiedMillis int,
	clock clock.Clock,
	logger logging.Logger,
) error {
	logger.Debugf("attempting to sync arbitrary file: %s", f.Name())
	path, err := filepath.Abs(f.Name())
	if err != nil {
		return errors.Wrap(err, "failed to get absolute path")
	}

	// Only sync non-datacapture files that have not been modified in the last
	// fileLastModifiedMillis to avoid uploading files that are being
	// to written to.
	info, err := os.Stat(path)
	if err != nil {
		return errors.Wrap(err, "stat failed")
	}
	if info.Size() == 0 {
		return errFileEmpty
	}

	timeSinceMod := clock.Since(info.ModTime())
	if timeSinceMod < time.Duration(fileLastModifiedMillis)*time.Millisecond {
		return errFileModifiedTooRecently
	}

	// We need to seek to the start of the file as if we have tried to upload this file
	// previously, the read offset of the file might not be at the beginning, which would
	// result in partial data loss when uploading to the cloud.
	// Fixes https://viam.atlassian.net/browse/DATA-3114
	pos, err := f.Seek(0, io.SeekStart)
	if err != nil {
		return errors.Wrap(err, "error trying to Seek to beginning of file")
	}

	if pos != 0 {
		return fmt.Errorf("error trying to seek to beginning of file %s: expected position 0, instead got to position %d", path, pos)
	}

	logger.Debugf("datasync.FileUpload request started for arbitrary file: %s", path)
	stream, err := conn.client.FileUpload(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating FileUpload client")
	}

	// Send metadata FileUploadRequest.
	logger.Debugf("datasync.FileUpload request sending metadata for arbitrary file: %s", path)
	if err := stream.Send(&v1.FileUploadRequest{
		UploadPacket: &v1.FileUploadRequest_Metadata{
			Metadata: &v1.UploadMetadata{
				PartId:        conn.partID,
				Type:          v1.DataType_DATA_TYPE_FILE,
				FileName:      path,
				FileExtension: filepath.Ext(f.Name()),
				Tags:          tags,
			},
		},
	}); err != nil {
		return errors.Wrap(err, "FileUpload failed sending metadata")
	}

	if err := sendFileUploadRequests(ctx, stream, f, path, logger); err != nil {
		return errors.Wrap(err, "FileUpload failed to sync")
	}

	logger.Debugf("datasync.FileUpload closing for arbitrary file: %s", path)
	_, err = stream.CloseAndRecv()
	return errors.Wrap(err, "FileUpload  CloseAndRecv failed")
}

func sendFileUploadRequests(
	ctx context.Context,
	stream v1.DataSyncService_FileUploadClient,
	f *os.File,
	path string,
	logger logging.Logger,
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
