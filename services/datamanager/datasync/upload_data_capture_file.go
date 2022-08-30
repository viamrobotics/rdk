package datasync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

func uploadDataCaptureFile(ctx context.Context, pt progressTracker, client v1.DataSyncServiceClient,
	md *v1.UploadMetadata, f *os.File,
) error {
	stream, err := client.Upload(ctx)
	if err != nil {
		return err
	}
	err = initDataCaptureUpload(ctx, f, pt, f.Name())
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}

	progressFileName := filepath.Join(pt.progressDir, filepath.Base(f.Name()))

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
		err := recvStream(cancelCtx, stream, pt, progressFileName)
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
	if err := pt.deleteProgressFile(progressFileName); err != nil {
		return err
	}

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
