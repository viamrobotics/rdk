package datasync

import (
	"context"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"io"
	"os"
	"path/filepath"
)

func uploadDataCaptureFile(ctx context.Context, client v1.DataSyncServiceClient, f *datacapture.File, partID string) error {
	// Send metadata upload request.
	md, err := f.ReadMetadata()
	if err != nil {
		return err
	}
	_ = &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: &v1.UploadMetadata{
				PartId:           partID,
				ComponentType:    md.GetComponentType(),
				ComponentName:    md.GetComponentName(),
				ComponentModel:   md.GetComponentModel(),
				MethodName:       md.GetMethodName(),
				Type:             md.GetType(),
				MethodParameters: md.GetMethodParameters(),
				Tags:             md.GetTags(),
				SessionId:        md.GetSessionId(),
			},
		},
	}

	// TODO: make unary upload call with whole contents of file. return error if receive error.
	return nil
}

func initDataCaptureUpload(ctx context.Context, f *os.File, pt progressTracker, dcFileName string) error {
	finfo, err := f.Stat()
	if err != nil {
		return err
	}
	// Get file progress to see if upload has been attempted. If yes, resume from upload progress point and if not,
	// create an upload progress file.
	progressFilePath := filepath.Join(pt.progressDir, filepath.Base(dcFileName))
	progressIndex, err := pt.getProgressFileIndex(progressFilePath)
	if errors.Is(err, os.ErrNotExist) {
		if err := pt.createProgressFile(progressFilePath); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return err
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
		next, err := datacapture.ReadNextSensorData(f)
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

func sendNextUploadRequest(ctx context.Context, f *os.File, stream v1.DataSyncService_UploadClient) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextSensorUploadRequest(ctx, f)
		if err != nil {
			return err
		}

		if err = stream.Send(uploadReq); err != nil {
			return err
		}
	}
	return nil
}

func recvStream(ctx context.Context, stream v1.DataSyncService_UploadClient,
	pt progressTracker, progressFile string,
) error {
	for {
		recvChannel := make(chan error)
		go func() {
			defer close(recvChannel)
			ur, err := stream.Recv()
			if err != nil {
				recvChannel <- err
				return
			}
			if err := pt.updateProgressFileIndex(progressFile, int(ur.GetRequestsWritten())); err != nil {
				recvChannel <- err
				return
			}
		}()

		select {
		case <-ctx.Done():
			return context.Canceled
		case e := <-recvChannel:
			if e != nil {
				if !errors.Is(e, io.EOF) {
					return e
				}
				return nil
			}
		}
	}
}

func sendStream(ctx context.Context, stream v1.DataSyncService_UploadClient,
	captureFile *os.File,
) error {
	// Loop until there is no more content to be read from file.
	for {
		err := sendNextUploadRequest(ctx, captureFile, stream)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	if err := stream.CloseSend(); err != nil {
		return err
	}
	return nil
}
