package datasync

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

func uploadArbitraryFile(ctx context.Context, client v1.DataSyncServiceClient, md *v1.UploadMetadata,
	f *os.File,
) error {
	stream, err := client.Upload(ctx)
	if err != nil {
		return err
	}
	// Send metadata upload request.
	req := &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: md,
		},
	}
	if err := stream.Send(req); err != nil {
		return errors.Wrap(err, "error while sending upload metadata")
	}

	// Loop until there is no more content to be read from file.
	for {
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextFileUploadRequest(ctx, f)
		// If the error is EOF, break from loop.
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, datacapture.EmptyReadingErr(filepath.Base(f.Name()))) {
			continue
		}
		// If there is any other error, return it.
		if err != nil {
			return err
		}

		if err = stream.Send(uploadReq); err != nil {
			return errors.Wrap(err, "error while sending uploadRequest")
		}
	}

	if err := f.Close(); err != nil {
		return err
	}

	// Close stream and receive response.
	if _, err := stream.CloseAndRecv(); err != nil {
		return errors.Wrap(err, "error when closing the stream and receiving the response from "+
			"sync service backend")
	}

	return nil
}

func getNextFileUploadRequest(ctx context.Context, f *os.File) (*v1.UploadRequest, error) {
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
		return &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_FileContents{
				FileContents: next,
			},
		}, nil
	}
}

func readNextFileChunk(f *os.File) (*v1.FileData, error) {
	byteArr := make([]byte, uploadChunkSize)
	numBytesRead, err := f.Read(byteArr)
	if numBytesRead < uploadChunkSize {
		byteArr = byteArr[:numBytesRead]
	}
	if err != nil {
		return nil, err
	}
	return &v1.FileData{Data: byteArr}, nil
}
