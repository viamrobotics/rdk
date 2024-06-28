package sync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
)

// UploadChunkSize defines the size of the data included in each message of a FileUpload stream.
var UploadChunkSize = 64 * 1024

// TODO: I'm pretty sure we should never need to check any of these things as they should have been checked when the file was enqueued
// what happens if any of these errors happen? Does it get retried forever?
func uploadArbitraryFile(
	ctx context.Context,
	f *os.File,
	conn cloudConn,
	tags []string,
	fileLastModifiedMillis int,
	clock clock.Clock,
) error {
	stream, err := conn.client.FileUpload(ctx)
	if err != nil {
		return err
	}

	path, err := filepath.Abs(f.Name())
	if err != nil {
		return err
	}

	// Only sync non-datacapture files that have not been modified in the last
	// fileLastModifiedMillis to avoid uploading files that are being
	// to written to.
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return errors.New("file is empty (0 bytes)")
	}

	timeSinceMod := clock.Since(info.ModTime())
	if timeSinceMod < time.Duration(fileLastModifiedMillis)*time.Millisecond {
		return errors.New("file modified too recently")
	}

	// Send metadata FileUploadRequest.
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
		return err
	}

	if err := sendFileUploadRequests(ctx, stream, f); err != nil {
		return errors.Wrapf(err, "error syncing %s", f.Name())
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		return errors.Wrapf(err, "received error response while syncing %s", f.Name())
	}

	return nil
}

func sendFileUploadRequests(ctx context.Context, stream v1.DataSyncService_FileUploadClient, f *os.File) error {
	// Loop until there is no more content to be read from file.
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

		if err = stream.Send(uploadReq); err != nil {
			return err
		}
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
