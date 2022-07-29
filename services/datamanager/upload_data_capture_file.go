package datamanager

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
)

func uploadDataCaptureFile(ctx context.Context, s *syncer, client v1.DataSyncService_UploadClient,
	md *v1.UploadMetadata, f *os.File,
) error {
	err := initDataCaptureUpload(ctx, f, s.progressTracker, f.Name(), md)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
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
	// Loop until there is no more content to be read from file.
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-progress:
			uploadResponse := <-progress
			if err := s.progressTracker.updateProgressFileIndex(filepath.Join(s.progressTracker.progressDir, filepath.
				Base(f.Name())), int(uploadResponse.GetRequestsWritten())); err != nil {
				return err
			}
		default:
			// Get the next UploadRequest from the file.
			uploadReq, err := getNextSensorUploadRequest(ctx, f)
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
	if err := client.CloseSend(); err != nil {
		return errors.Wrap(err, "error when closing the stream")
	}

	return nil
}

func initDataCaptureUpload(ctx context.Context, f *os.File, pt progressTracker, dcFileName string, md *v1.UploadMetadata) error {
	finfo, err := f.Stat()
	if err != nil {
		return err
	}
	// Get file progress to see if upload has been attempted. If yes, resume from upload progress point and if not,
	// create an upload progress file.
	progressFilePath := filepath.Join(pt.progressDir, filepath.Base(dcFileName))
	progressIndex, err := pt.getProgressFileIndex(progressFilePath)
	if err != nil {
		return err
	}
	if progressIndex == 0 {
		if err := pt.createProgressFile(progressFilePath); err != nil {
			return err
		}
	}

	// Sets the next file pointer to the next sensordata message that needs to be uploaded.
	for i := 0; i < progressIndex; i++ {
		if _, err := getNextSensorUploadRequest(ctx, f); err != nil {
			return err
		}
	}

	// Check remaining data capture file contents so we know whether to continue upload process.
	currentOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if currentOffset == finfo.Size() {
		return io.EOF
	}
	return nil
}

func getNextSensorUploadRequest(ctx context.Context, f *os.File) (*v1.UploadRequest, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:

		// Get the next sensor data reading from file, check for an error.
		next, err := readNextSensorData(f)
		if err != nil {
			return nil, err
		}
		// Otherwise, return an UploadRequest and no error.
		return &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_SensorContents{
				SensorContents: next,
			},
		}, nil
	}
}
