package datasync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

// TODO for bidi partial uploads:
//      - Have goroutine waiting on Recv on stream (select with cancel context). On recv:
//        - Update progress index
//        - If EOF: Return nil. we know all messages were sent and received becuse errors are a response type, so
//                       are sent serially with other responses
//        - If other error: We actually returned an error from our server. Determine how to handle individually. For now
//                      a blanket "return err" (triggering restarts) is probably fine
//      - Have sends happening in goroutine
//          - If EOF, send EOF and Close (not recv!) to server. This will trigger server to send final ack then EOF.
//          - If other error, return error
//      Wait for both goroutines. If error, return error. If none, return nil.
//
// TODO
//      Some principles to keep in mind:
//        - The thread/goroutine sending on a channel/grpc connection should be the one closing it
//        - If some thing is blocking or long running, it should probably be passed a context so it can be cancelled
func uploadDataCaptureFile(ctx context.Context, pt progressTracker, client v1.DataSyncServiceClient,
	md *v1.UploadMetadata, f *os.File,
) error {
	// TODO: make cancel context (child of ctx), cancelCtx, cancelFn := context.WithCancel(ctx)
	// Add ctx.Done to all select statements
	//  In both goroutines, if error, cancel other goroutine by calling cancelCtx

	//TODO: another maybe-performance-optimal blah TODO: see if you can defer some of the close stuff, because repeated a lot. not sure if can. Find out

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

	// activeBackgroundWorkers ensures stream.Recv() goroutine & stream.Send() goroutine terminate before we return
	// from the enclosing uploadDataCaptureFile function.
	var activeBackgroundWorkers sync.WaitGroup

	retRecvUploadResponse := make(chan error, 1)
	activeBackgroundWorkers.Add(1)
	go func() {
		defer activeBackgroundWorkers.Done()
		defer close(retRecvUploadResponse)
		for {
			// if ctx.Err() != nil {
			// 	retRecvUploadResponse <- ctx.Err()
			// 	close(retRecvUploadResponse)
			// 	return
			// }
			select {
			case <-ctx.Done():
				retRecvUploadResponse <- ctx.Err()
				return
			default:
				ur, err := stream.Recv()
				if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
					retRecvUploadResponse <- err
					return
				}
				if err != nil {
					retRecvUploadResponse <- errors.Errorf("Unable to receive UploadResponse from server: %v", err)
					return
				} else {
					if err := pt.updateProgressFileIndex(progressFileName, int(ur.GetRequestsWritten())); err != nil {
						retRecvUploadResponse <- err
						return
					}
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
		defer stream.CloseSend()
		for {
			// if err := ctx.Err(); err != nil {
			// 	retSendingUploadReqs <- err
			// 	close(retSendingUploadReqs)
			// 	_ = stream.CloseSend()
			// 	return
			// }
			select {
			case <-ctx.Done():
				if ctx.Err() != nil {
					retSendingUploadReqs <- ctx.Err()
				}
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
					return
				}

				if err = stream.Send(uploadReq); err != nil {
					retSendingUploadReqs <- err
					return
				}

			}
		}
	}()
	activeBackgroundWorkers.Wait()

	if err := <-retRecvUploadResponse; err != nil {
		return errors.Errorf("Error when trying to recv UploadResponse from server: %v", err)
	}
	if err := <-retSendingUploadReqs; err != nil {
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
			return err
		}
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
