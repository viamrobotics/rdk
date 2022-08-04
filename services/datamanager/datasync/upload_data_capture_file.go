package datasync

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
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

	// Loop until there is no more content to be read from file.
	for {
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextSensorUploadRequest(ctx, f)
		// If the error is EOF, break from loop.
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, EmptyReadingErr(filepath.Base(f.Name()))) {
			continue
		}
		// If there is any other error, return it.
		if err != nil {
			return err
		}

		if err = client.Send(uploadReq); err != nil {
			return errors.Wrap(err, "error while sending uploadRequest")
		}
		if err := s.progressTracker.incrementProgressFileIndex(filepath.Join(s.progressTracker.progressDir, filepath.
			Base(f.Name()))); err != nil {
			return err
		}
	}

	if err := f.Close(); err != nil {
		return err
	}

	// Close stream and receive response.
	if _, err := client.CloseAndRecv(); err != nil {
		return errors.Wrap(err, "error when closing the stream and receiving the response from "+
			"datasync service backend")
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

// readNextSensorData reads sensorData sequentially from a data capture file. It assumes the file offset is already
// pointing at the beginning of series of SensorData in the file. This is accomplished by first calling
// ReadDataCaptureMetadata.
func readNextSensorData(f *os.File) (*v1.SensorData, error) {
	r := &v1.SensorData{}
	if _, err := pbutil.ReadDelimited(f, r); err != nil {
		return nil, err
	}

	// Ensure we construct and return a SensorData value for tabular data when the tabular data's fields and
	// corresponding entries are not nil. Otherwise, return io.EOF error and nil.
	if r.GetBinary() == nil && r.GetStruct() == nil {
		return r, EmptyReadingErr(filepath.Base(f.Name()))
	}
	return r, nil
}
