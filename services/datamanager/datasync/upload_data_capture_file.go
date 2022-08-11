package datasync

import (
	"context"
	"io"
	"os"
	"path/filepath"

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

	// Send metadata upload request.
	req := &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: md,
		},
	}
	if err := stream.Send(req); err != nil {
		return errors.Wrap(err, "error while sending upload metadata")
	}

	// Loop until there is no more content to be read from file.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Get the next UploadRequest from the file.
			uploadReq, err := getNextSensorUploadRequest(ctx, f)

			// If the error is EOF, we completed successfully.
			if errors.Is(err, io.EOF) {
				if _, err := stream.CloseAndRecv(); err != nil {
					if !errors.Is(err, io.EOF) {
						return err
					}
				}

				if err := pt.deleteProgressFile(filepath.Join(pt.progressDir,
					filepath.Base(filepath.Base(f.Name())))); err != nil {
					return err
				}
				return nil
			}
			if errors.Is(err, datacapture.EmptyReadingErr(filepath.Base(f.Name()))) {
				continue
			}
			if err != nil {
				return err
			}

			if err = stream.Send(uploadReq); err != nil {
				return err
			}
			if err := pt.incrementProgressFileIndex(filepath.Join(pt.progressDir, filepath.
				Base(f.Name()))); err != nil {
				return err
			}
		}
	}
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
