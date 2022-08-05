package datasync

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

var debug = true

func uploadDataCaptureFile(ctx context.Context, s *syncer, client v1.DataSyncService_UploadClient,
	md *v1.UploadMetadata, f *os.File,
) error {
	count := 0
	if debug {
		fmt.Println("CLIENT POINT: Begin.")
	}
	err := initDataCaptureUpload(ctx, f, s.progressTracker, f.Name(), md)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	if debug {
		fmt.Println("CLIENT POINT: Initialized upload.")
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
	count++
	if debug {
		fmt.Println("CLIENT POINT: Sent Request #", fmt.Sprint(count), ".")
	}

	// Create channel between goroutine that's waiting for UploadResponse and the main goroutine which is persisting
	// file upload progress to disk.
	progress := make(chan v1.UploadResponse, 1)

	// activeBackgroundWorkers ensures upload func waits for forked goroutines to terminate before exiting.
	var activeBackgroundWorkers sync.WaitGroup

	activeBackgroundWorkers.Add(1)
	go func() {
		if debug {
			fmt.Println("CLIENT POINT: Started goroutine to wait for server's response.")
		}
		shouldBreak := false
		defer activeBackgroundWorkers.Done()
		for {
			if debug {
				fmt.Println("CLIENT POINT: Starting 'select-loop' while waiting for response.")
			}
			if shouldBreak {
				shouldBreak = false
				break
			}
			select {
			case <-ctx.Done():
				if debug {
					fmt.Println("CLIENT POINT: Cancelled context while waiting on upload response from server.")
				}
				shouldBreak = true
			default:
				ur, err := client.Recv()
				if err == io.EOF {
					if debug {
						fmt.Println("CLIENT POINT: End of file error from client.")
					}
					close(progress)
					shouldBreak = true
				} else {
					if err != nil {
						log.Fatalf("Unable to receive UploadResponse from server %v", err)
					} else {
						if debug {
							fmt.Println("CLIENT POINT: Received progress (" + fmt.Sprint(ur.GetRequestsWritten()) +
								" messages written) from server.")
						}
						progress <- *ur
						if debug {
							fmt.Println("CLIENT POINT: Successfully sent progress on channel.")
						}
					}
				}
			}
		}
	}()

	eof := false
	// Loop until there is no more content to be read from file.
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			if debug {
				fmt.Println("CLIENT POINT: Cancelled context.")
			}
			return context.Canceled
		case uploadResponse := <-progress:
			if debug {
				fmt.Println("CLIENT POINT: Received progress from channel.")
			}
			if err := s.progressTracker.updateProgressFileIndex(filepath.Join(s.progressTracker.progressDir, filepath.
				Base(f.Name())), int(uploadResponse.GetRequestsWritten())); err != nil {
				return err
			}
			if debug {
				fmt.Println("CLIENT POINT: Updated progress file.")
			}
		default:
			if debug {
				fmt.Println("CLIENT POINT: Getting next upload request to send to server.")
			}
			// Get the next UploadRequest from the file.
			uploadReq, err := getNextSensorUploadRequest(ctx, f)
			if uploadReq == nil {
				if debug {
					fmt.Println("CLIENT POINT: Upload request is nil.")
				}
			}
			// If the error is EOF, break from loop.
			if errors.Is(err, io.EOF) || uploadReq == nil {
				eof = true
				break
			}
			if errors.Is(err, datacapture.EmptyReadingErr(filepath.Base(f.Name()))) {
				continue
			}
			// If there is any other error, return it.
			if err != nil {
				return err
			}

			if err = client.Send(uploadReq); err != nil {
				return errors.Wrap(err, "error while sending uploadRequest")
			}
			count++

			if debug {
				fmt.Println("CLIENT POINT: Sent upload request #", fmt.Sprint(count), " to server: ",
					string(uploadReq.GetSensorContents().GetBinary()))
			}
		}
		if eof {
			eof = false
			break
		}
	}
	activeBackgroundWorkers.Wait()

	if err := f.Close(); err != nil {
		return err
	}

	// Close stream and receive response.
	if err := client.CloseSend(); err != nil {
		return errors.Wrap(err, "error when closing the stream")
	}
	if debug {
		fmt.Println("CLIENT POINT: Successfully closed the client stream.")
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
