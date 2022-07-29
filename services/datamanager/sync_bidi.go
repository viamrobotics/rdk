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

func (s *syncer) uploadFile(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partID string) error {
	//nolint:gosec
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "error while opening file %s", path)
	}

	// Resets file pointer to ensure we are reading from beginning of file.
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}

	md, err := getMetadata(f, partID)
	if err != nil {
		return err
	}

	var getNextUploadRequest getNextUploadRequestFunc
	var processUploadRequest processUploadRequestFunc
	switch md.GetType() {
	case v1.DataType_DATA_TYPE_BINARY_SENSOR, v1.DataType_DATA_TYPE_TABULAR_SENSOR:
		err = initDataCaptureUpload(ctx, f, s.progressTracker, f.Name(), md)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		getNextUploadRequest = getNextSensorUploadRequest
		processUploadRequest = func(client v1.DataSyncService_UploadClient, req *v1.UploadRequest) error {
			return sendReqAndUpdateProgress(client, req, s.progressTracker, f.Name())
		}

	case v1.DataType_DATA_TYPE_FILE:
		getNextUploadRequest = getNextFileUploadRequest
		processUploadRequest = sendReq
	case v1.DataType_DATA_TYPE_UNSPECIFIED:
		return errors.New("no data type specified in upload metadata")
	default:
		return errors.New("no data type specified in upload metadata")
	}

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
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-progress:
			uploadResponse := <-progress
			requestsWritten := uploadResponse.GetRequestsWritten()
			fmt.Println("TODO: deal with requestsWritten: ", requestsWritten)
		default:
			// Loop until there is no more content to be read from file.
			// Get the next UploadRequest from the file.
			uploadReq, err := getNextUploadRequest(ctx, f)
			// If the error is EOF, break from loop.
			if errors.Is(err, io.EOF) {
				eof = true
				break
			}
			if errors.Is(err, emptyReadingErr(filepath.Base(f.Name()))) {
				continue
			}
			// If there is any other error, return it.
			if err != nil {
				return err
			}
			// Send upload request to server and persist file upload progress on disk if necessary.
			if err = processUploadRequest(client, uploadReq); err != nil {
				return err
			}
		}
		if eof {
			break
		}
	}

	if err = f.Close(); err != nil {
		return err
	}

	// Close the send direction of the stream.
	if err = client.CloseSend(); err != nil {
		return errors.Wrap(err, "error when closing the stream and receiving the response from "+
			"sync service backend")
	}

	return nil
}
