package datasync

import (
	"context"
	"io"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

func uploadDataCaptureFile(ctx context.Context, pt progressTracker, client v1.DataSyncServiceClient,
	partID string, f *datacapture.File,
) error {
	stream, err := client.Upload(ctx)
	if err != nil {
		return err
	}

	captureMD := f.ReadMetadata()

	md := &v1.UploadMetadata{
		PartId:           partID,
		ComponentType:    captureMD.GetComponentType(),
		ComponentName:    captureMD.GetComponentName(),
		ComponentModel:   captureMD.GetComponentModel(),
		MethodName:       captureMD.GetMethodName(),
		Type:             captureMD.GetType(),
		FileName:         filepath.Base(f.GetPath()),
		MethodParameters: captureMD.GetMethodParameters(),
		FileExtension:    captureMD.GetFileExtension(),
		Tags:             captureMD.GetTags(),
	}

	err = initDataCaptureUpload(f, pt)
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
	if err := stream.Send(req); err != nil {
		return errors.Wrap(err, "error while sending upload metadata")
	}

	activeBackgroundWorkers := &sync.WaitGroup{}
	errChannel := make(chan error, 1)

	// Create cancelCtx so if either the recv or send goroutines error, we can cancel the other one.
	cancelCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	// Start a goroutine for recving acks back from the server.
	activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer activeBackgroundWorkers.Done()
		err := recvStream(cancelCtx, stream, pt, f)
		if err != nil {
			errChannel <- err
			cancelFn()
		}
	})

	// Start a goroutine for sending SensorData to the server.
	activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer activeBackgroundWorkers.Done()
		err := sendStream(cancelCtx, stream, f)
		if err != nil {
			errChannel <- err
			cancelFn()
		}
	})

	activeBackgroundWorkers.Wait()
	close(errChannel)

	for err := range errChannel {
		return err
	}

	// Upload is complete, delete the corresponding progress file on disk.
	if err := pt.deleteProgressFile(f); err != nil {
		return err
	}

	return nil
}

func initDataCaptureUpload(f *datacapture.File, pt progressTracker) error {
	// Get file progress to see if upload has been attempted. If yes, resume from upload progress point and if not,
	// create an upload progress file.
	progressIndex, err := pt.getProgress(f)
	if err != nil {
		return err
	}

	// Sets the next file pointer to the next sensordata message that needs to be uploaded.
	for i := 0; i < progressIndex; i++ {
		if _, err := f.ReadNext(); err != nil {
			return err
		}
	}

	return nil
}

func getNextSensorUploadRequest(ctx context.Context, f *datacapture.File) (*v1.UploadRequest, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
		// Get the next sensor data reading from file, check for an error.
		next, err := f.ReadNext()
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

func sendNextUploadRequest(ctx context.Context, f *datacapture.File, stream v1.DataSyncService_UploadClient) error {
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
	pt progressTracker, f *datacapture.File,
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
			if err := pt.updateProgress(f, int(ur.GetRequestsWritten())); err != nil {
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
	f *datacapture.File,
) error {
	// Loop until there is no more content to be read from file.
	for {
		err := sendNextUploadRequest(ctx, f, stream)
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
