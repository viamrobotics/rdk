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

	cancelCtx, cancelFn := context.WithCancel(ctx)
	var activeBackgroundWorkers sync.WaitGroup

	retRecvUploadResponse := make(chan error, 1)
	activeBackgroundWorkers.Add(1)
	go func() {
		defer activeBackgroundWorkers.Done()
		defer close(retRecvUploadResponse)
		for {
			select {
			case <-cancelCtx.Done():
				retRecvUploadResponse <- cancelCtx.Err()
				cancelFn()
				return
			default:
				ur, err := stream.Recv()
				if errors.Is(err, io.EOF) {
					return
				}
				if err != nil {
					fmt.Println("received error from server")
					retRecvUploadResponse <- err
					cancelFn()
					return
				}
				if err := pt.updateProgressFileIndex(progressFileName, int(ur.GetRequestsWritten())); err != nil {
					retRecvUploadResponse <- err
					cancelFn()
					return
				}
			}
		}
	}()

	retSendingUploadReqs := make(chan error, 1)
	activeBackgroundWorkers.Add(1)
	go func() {
		// Loop until there is no more content to be read from file.
		defer activeBackgroundWorkers.Done()
		defer close(retSendingUploadReqs)
		// Do not check error of stream close send because it is always following the error check
		// of the wider function execution.
		//nolint: errcheck
		defer stream.CloseSend()
		for {
			select {
			case <-cancelCtx.Done():
				if cancelCtx.Err() != nil {
					retSendingUploadReqs <- cancelCtx.Err()
				}
				cancelFn()
				return
			default:
				// Get the next UploadRequest from the file.
				uploadReq, err := getNextSensorUploadRequest(ctx, f)

				// If the error is EOF, we completed successfully.
				if errors.Is(err, io.EOF) {
					// Close the stream to server.
					if err = stream.CloseSend(); err != nil {
						retSendingUploadReqs <- errors.Errorf("error when closing the stream: %v", err)
					}
					return
				}
				if errors.Is(err, datacapture.EmptyReadingErr(filepath.Base(f.Name()))) {
					continue
				}
				if err != nil {
					retSendingUploadReqs <- err
					cancelFn()
					return
				}

				if err = stream.Send(uploadReq); err != nil {
					fmt.Println("received error when sending to server")
					retSendingUploadReqs <- err
					cancelFn()
					return
				}
			}
		}
	}()
	activeBackgroundWorkers.Wait()

	if err := <-retRecvUploadResponse; err != nil {
		fmt.Printf("Error when trying to recv UploadResponse from server: %v", err)
		return errors.Errorf("Error when trying to recv UploadResponse from server: %v", err)
	}
	if err := <-retSendingUploadReqs; err != nil {
		fmt.Printf("Error when trying to send UploadRequest to server: %v", err)
		return errors.Errorf("Error when trying to send UploadRequest to server: %v", err)
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
	if err != nil {
		return err
	}
	if progressIndex == 0 {
		if err := pt.createProgressFile(progressFilePath); err != nil {
			fmt.Println("error creating progress file")
			fmt.Println(err)
			return err
		}
		fmt.Println("created progress file " + progressFilePath)
		return nil
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
		fmt.Println("client context done: " + ctx.Err().Error())
		return nil, ctx.Err()
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
