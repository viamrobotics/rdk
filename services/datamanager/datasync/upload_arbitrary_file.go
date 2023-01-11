package datasync

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
)

// UploadChunkSize defines the size of the data included in each message of a FileUpload stream.
var UploadChunkSize = 64 * 1024

func uploadArbitraryFile(ctx context.Context, client v1.DataSyncServiceClient, f *os.File, partID string) error {
	stream, err := client.FileUpload(ctx)
	if err != nil {
		return err
	}

	md := &v1.UploadMetadata{
		PartId:        partID,
		Type:          v1.DataType_DATA_TYPE_FILE,
		FileName:      filepath.Base(f.Name()),
		FileExtension: filepath.Ext(f.Name()),
	}

	// Send metadata FileUploadRequest.
	req := &v1.FileUploadRequest{
		UploadPacket: &v1.FileUploadRequest_Metadata{
			Metadata: md,
		},
	}
	if err := stream.Send(req); err != nil {
		return err
	}

	if err := sendFileUploadRequests(ctx, stream, f); err != nil {
		return errors.Wrapf(err, "error syncing %s", f.Name())
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		return errors.Wrapf(err, "received error response while syncing %s", f.Name())
	}

	return nil
}

func sendFileUploadRequests(ctx context.Context, stream v1.DataSyncService_FileUploadClient, f *os.File) error {
	//nolint:errcheck
	defer stream.CloseSend()
	// Loop until there is no more content to be read from file.
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
			// Get the next UploadRequest from the file.
			uploadReq, err := getNextFileUploadRequest(ctx, f)

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
}

func getNextFileUploadRequest(ctx context.Context, f *os.File) (*v1.FileUploadRequest, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
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
}

func readNextFileChunk(f *os.File) (*v1.FileData, error) {
	byteArr := make([]byte, UploadChunkSize)
	numBytesRead, err := f.Read(byteArr)
	if numBytesRead < UploadChunkSize {
		byteArr = byteArr[:numBytesRead]
	}
	if err != nil {
		return nil, err
	}
	return &v1.FileData{Data: byteArr}, nil
}
