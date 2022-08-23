package datasync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

// TODO: Have two goroutines. If one errors and exits, it should cancel the other one. This is important because if the
//       recv routine fails but the send continues, the server will continue to send ACKs (and persist data), but we
//       won't be recving those ACKs, so duplicate data will be sent on the next run.

// TODO: clean up this messy ass code
func uploadDataCaptureFile(ctx context.Context, pt progressTracker, client v1.DataSyncServiceClient,
	md *v1.UploadMetadata, f *os.File,
) error {
	fmt.Println("started upload")
	stream, err := client.Upload(ctx)
	if err != nil {
		return err
	}
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	err = initDataCaptureUpload(ctx, f, pt, f.Name())
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}

	progressFileName := filepath.Join(pt.progressDir,
		filepath.Base(f.Name()))

	// Send metadata upload request.
	req := &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: md,
		},
	}
	if err := stream.Send(req); err != nil {
		return errors.Wrap(err, "error while sending upload metadata")
	}

	var activeBackgroundWorkers sync.WaitGroup
	errChannel := make(chan error, 1)

	// Start a goroutine for recving acks back from the server.
	activeBackgroundWorkers.Add(1)
	// TODO: make this a named function
	go func() {
		defer fmt.Println("recv returned")
		defer activeBackgroundWorkers.Done()
		for {
			recvChannel := make(chan error)
			go func() {
				defer close(recvChannel)
				fmt.Println("waiting on recv")
				ur, err := stream.Recv()
				if err != nil {
					fmt.Println("received error from server")
					fmt.Println(err)
					recvChannel <- err
					return
				}
				fmt.Println("received ack")
				if err := pt.updateProgressFileIndex(progressFileName, int(ur.GetRequestsWritten())); err != nil {
					recvChannel <- err
					return
				}
			}()

			select {
			case <-ctx.Done():
				errChannel <- context.Canceled
				return
			case e := <-recvChannel:
				if e != nil {
					if !errors.Is(e, io.EOF) {
						fmt.Println("received non EOF error from server")
						fmt.Println(e)
						fmt.Println(e == nil)
						errChannel <- e
						cancelFn()
					} else {
						return
					}
				}
			}
		}
	}()

	// Start a goroutine for sending SensorData to the server.
	activeBackgroundWorkers.Add(1)
	// TODO: make this a named function
	go func() {
		defer fmt.Println("send returned")
		// Loop until there is no more content to be read from file.
		defer activeBackgroundWorkers.Done()
		for {
			fmt.Println("sending upload request")
			err := sendNextUploadRequest(ctx, f, stream)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				fmt.Println("error when reading ")
				errChannel <- err
				cancelFn()
				break
			}
			fmt.Println("sent upload request")
		}
		if err = stream.CloseSend(); err != nil {
			errChannel <- err
		}
	}()
	activeBackgroundWorkers.Wait()
	fmt.Println("done waiting")

	close(errChannel)

	// TODO: should probably combine errors
	for err := range errChannel {
		fmt.Println("received error")
		fmt.Println(err.Error())
		if err == nil {
			continue
		}
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

		if errors.Is(err, datacapture.EmptyReadingErr(filepath.Base(f.Name()))) {
			return nil
		}
		if err != nil {
			return err
		}

		if err = stream.Send(uploadReq); err != nil {
			return err
		}
	}
	return nil
}
