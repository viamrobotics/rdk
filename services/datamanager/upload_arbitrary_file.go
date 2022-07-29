package datamanager

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
)

func uploadArbitraryFile(ctx context.Context, s *syncer, client v1.DataSyncService_UploadClient, md *v1.UploadMetadata,
	f *os.File,
) error {
	// Send metadata upload request.
	req := &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: md,
		},
	}
	if err := client.Send(req); err != nil {
		return errors.Wrap(err, "error while sending upload metadata")
	}

	// Create channel between goroutine that's waiting for UploadResponse and the main goroutine which is persisting
	// file upload progress to disk.
	progress := make(chan v1.UploadResponse)
	go func() {
		for {
			ur, err := client.Recv()
			if err == io.EOF {
				close(progress)
				return
			}
			if err != nil {
				log.Fatalf("Unable to receive UploadResponse from server %v", err)
			}
			progress <- *ur
		}
	}()

	eof := false
	// Loop until there is no more content to be read from file.
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-progress:
			uploadResponse := <-progress
			requestsWritten := uploadResponse.GetRequestsWritten()
			fmt.Println("TODO: deal with requestsWritten: ", requestsWritten)
		default:
			// Get the next UploadRequest from the file.
			uploadReq, err := getNextFileUploadRequest(ctx, f)
			// If the error is EOF, break from loop.
			if errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, emptyReadingErr(filepath.Base(f.Name()))) {
				continue
			}
			// If there is any other error, return it.
			if err != nil {
				return err
			}

			if err = client.Send(uploadReq); err != nil {
				return errors.Wrap(err, "error while sending uploadRequest")
			}
		}
		if eof {
			break
		}
	}

	if err := f.Close(); err != nil {
		return err
	}

	// Close stream and receive response.
	if _, err := client.Recv(); err != nil {
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
