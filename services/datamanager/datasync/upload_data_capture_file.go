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

	// TODO: use single error channel for these two
	var activeBackgroundWorkers sync.WaitGroup

	errChannel := make(chan error, 1)
	activeBackgroundWorkers.Add(1)
	go func() {
		defer activeBackgroundWorkers.Done()
		for {
			recvChannel := make(chan error)
			go func() {
				defer close(recvChannel)
				ur, err := stream.Recv()
				if err != nil {
					recvChannel <- err
					return
				}
				if err := pt.updateProgressFileIndex(progressFileName, int(ur.GetRequestsWritten())); err != nil {
					recvChannel <- err
					return
				}
				recvChannel <- nil
			}()

			select {
			case <-ctx.Done():
				fmt.Println("ctx done in recv")
				errChannel <- context.Canceled
				return
			case e := <-recvChannel:
				if e != nil {
					fmt.Println("reveived error on recv channel ")
					errChannel <- e
					return
				}
				//errChannel <- e
			}
		}
	}()

	activeBackgroundWorkers.Add(1)
	go func() {
		// Loop until there is no more content to be read from file.
		defer activeBackgroundWorkers.Done()
		// Do not check error of stream close send because it is always following the error check
		// of the wider function execution.
		//nolint: errcheck
		defer stream.CloseSend()
		for {
			select {
			case <-ctx.Done():
				fmt.Println("ctx done in send")
				errChannel <- context.Canceled
				return
			default:
				// Get the next UploadRequest from the file.
				uploadReq, err := getNextSensorUploadRequest(ctx, f)

				// If the error is EOF, we completed successfully.
				if errors.Is(err, io.EOF) {
					// Close the stream to server.
					if err = stream.CloseSend(); err != nil {
						errChannel <- errors.Errorf("error when closing the stream: %v", err)
					}
					return
				}
				if errors.Is(err, datacapture.EmptyReadingErr(filepath.Base(f.Name()))) {
					continue
				}
				if err != nil {
					errChannel <- err
					return
				}

				if err = stream.Send(uploadReq); err != nil {
					fmt.Println("received error when sending to server")
					errChannel <- err
					return
				}
				fmt.Println("sent " + uploadReq.String() + " to server")
			}
		}
	}()
	activeBackgroundWorkers.Wait()

	close(errChannel)

	// TODO: maybe combine error?
	for err := range errChannel {
		fmt.Println("entered err channel loop")
		if err == nil {
			fmt.Println("error is nil")
			continue
		}
		if !errors.Is(err, io.EOF) {
			fmt.Println("got non eof error " + err.Error())
			return err
		}
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
		// TODO: is this right?
		fmt.Println("client context done: " + ctx.Err().Error())
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
