package datamanager

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	goutils "go.viam.com/utils"
)

var (
	initialWaitTime        = time.Second
	retryExponentialFactor = 2
	maxRetryInterval       = time.Hour
	// Chunk size set at 32 kiB, this is 32768 Bytes.
	uploadChunkSize = 32768
)

func emptyReadingErr(fileName string) error {
	return errors.Errorf("%s contains SensorData containing no data", fileName)
}

// syncer is responsible for enqueuing files in captureDir and uploading them to the cloud.
type syncManager interface {
	Sync(paths []string)
	Close()
}

// syncer is responsible for uploading files in captureDir to the cloud.
type syncer struct {
	partID            string
	client            v1.DataSyncService_UploadClient
	logger            golog.Logger
	ProgressTracker   ProgressTracker
	uploadFn          uploadFn
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

type (
	getNextRequestFn       func(context.Context, *os.File) (*v1.UploadRequest, error)
	processUploadRequestFn func(v1.DataSyncService_UploadClient, *v1.UploadRequest, ProgressTracker, string) error
	uploadFn               func(context.Context, ProgressTracker,
		v1.DataSyncService_UploadClient, string, string) error
)

// TODO DATA-206: instantiate a client
// newSyncer returns a new syncer. If a nil uploadFunc is passed, the default viamUpload is used.
func newSyncer(logger golog.Logger, uploadFunc uploadFn, partID string) *syncer {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	ret := syncer{
		logger: logger,
		ProgressTracker: ProgressTracker{
			lock: &sync.Mutex{},
			m:    make(map[string]struct{}),
		},
		backgroundWorkers: sync.WaitGroup{},
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
		partID:            partID,
	}
	if uploadFunc == nil {
		uploadFunc = uploadFile
		// uploadFunc = uploadFile
	}
	ret.uploadFn = uploadFunc
	// nolint
	initProgressDir()
	return &ret
}

// Create progress directory in filesystem if it does not already exist.
func initProgressDir() error {
	if _, err := os.Stat(progressDir); os.IsNotExist(err) {
		err = os.Mkdir(progressDir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close closes all resources (goroutines) associated with s.
func (s *syncer) Close() {
	s.cancelFunc()
	s.backgroundWorkers.Wait()
}

func (s *syncer) upload(ctx context.Context, path string) {
	if s.ProgressTracker.inProgress(path) {
		return
	}

	s.ProgressTracker.mark(path)
	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer s.backgroundWorkers.Done()
		uploadErr := exponentialRetry(
			ctx,
			func(ctx context.Context) error { return s.uploadFn(ctx, s.ProgressTracker, s.client, path, s.partID) },
			s.logger,
		)
		if uploadErr != nil {
			return
		}

		// Delete the file and indicate that the upload is done.
		if err := os.Remove(path); err != nil {
			s.logger.Errorw("error while deleting file", "error", err)
		} else {
			s.ProgressTracker.unmark(path)
			if err := s.ProgressTracker.deleteProgressFile(path); err != nil {
				return
			}
		}
	})
}

func (s *syncer) Sync(paths []string) {
	for _, p := range paths {
		s.upload(s.cancelCtx, p)
	}
}

// exponentialRetry calls fn, logs any errors, and retries with exponentially increasing waits from initialWait to a
// maximum of maxRetryInterval.
func exponentialRetry(cancelCtx context.Context, fn func(cancelCtx context.Context) error, log golog.Logger) error {
	// Only create a ticker and enter the retry loop if we actually need to retry.
	if err := fn(cancelCtx); err == nil {
		return nil
	}

	// First call failed, so begin exponentialRetry with a factor of retryExponentialFactor
	nextWait := initialWaitTime
	ticker := time.NewTicker(nextWait)
	for {
		if err := cancelCtx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Errorw("context closed unexpectedly", "error", err)
			}
			return err
		}
		select {
		// If cancelled, return nil.
		case <-cancelCtx.Done():
			ticker.Stop()
			return cancelCtx.Err()
		// Otherwise, try again after nextWait.
		case <-ticker.C:
			if err := fn(cancelCtx); err != nil {
				// If error, retry with a new nextWait.
				log.Errorw("error while uploading file", "error", err)
				ticker.Stop()
				nextWait = getNextWait(nextWait)
				ticker = time.NewTicker(nextWait)
				continue
			}
			// If no error, return.
			ticker.Stop()
			return nil
		}
	}
}

func getNextWait(lastWait time.Duration) time.Duration {
	if lastWait == time.Duration(0) {
		return initialWaitTime
	}
	nextWait := lastWait * time.Duration(retryExponentialFactor)
	if nextWait > maxRetryInterval {
		return maxRetryInterval
	}
	return nextWait
}

func uploadFile(ctx context.Context, pt ProgressTracker, client v1.DataSyncService_UploadClient, path string, partID string) error {
	//nolint
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "error while opening file %s", path)
	}
	// Resets file pointer.
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}

	var md *v1.UploadMetadata
	if isDataCaptureFile(f) {
		captureMD, err := readDataCaptureMetadata(f)
		if err != nil {
			return err
		}
		md = &v1.UploadMetadata{
			PartId:           partID,
			ComponentType:    captureMD.GetComponentType(),
			ComponentName:    captureMD.GetComponentName(),
			MethodName:       captureMD.GetMethodName(),
			Type:             captureMD.GetType(),
			FileName:         filepath.Base(f.Name()),
			MethodParameters: captureMD.GetMethodParameters(),
		}
	} else {
		md = &v1.UploadMetadata{
			PartId:   partID,
			Type:     v1.DataType_DATA_TYPE_FILE,
			FileName: filepath.Base(f.Name()),
		}
	}

	// Construct the Metadata
	req := &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: md,
		},
	}
	if err := client.Send(req); err != nil {
		return errors.Wrap(err, "error while sending upload metadata")
	}

	var getNextRequestFn getNextRequestFn
	var processUploadRequestFn processUploadRequestFn

	switch md.GetType() {
	case v1.DataType_DATA_TYPE_BINARY_SENSOR, v1.DataType_DATA_TYPE_TABULAR_SENSOR:
		if err := initDataCaptureUpload(ctx, f, pt, f.Name(), md); err != nil {
			return err
		}
		getNextRequestFn = getNextSensorUploadRequest
		processUploadRequestFn = sendClientUpdateProgress
	case v1.DataType_DATA_TYPE_FILE:
		getNextRequestFn = getNextFileUploadRequest
		processUploadRequestFn = sendClient
	case v1.DataType_DATA_TYPE_UNSPECIFIED:
		return errors.New("no data type specified in upload metadata")
	default:
		return errors.New("no data type specified in upload metadata")
	}

	// Loop until there is no more content to be read from file.
	for {
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextRequestFn(ctx, f)
		// If the error is EOF, break from loop.
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, emptyReadingErr(filepath.Base(f.Name()))) {
			continue
		}
		// If there is any other error, return it.
		if err != nil {
			return err
		}
		// Finally, send the UploadRequest to the client.
		if err = processUploadRequestFn(client, uploadReq, pt, f.Name()); err != nil {
			return err
		}
	}

	if err = f.Close(); err != nil {
		return err
	}
	// Close stream and receive response.
	if _, err = client.CloseAndRecv(); err != nil {
		return errors.Wrap(err, "error when closing the stream and receiving the response from "+
			"sync service backend")
	}

	return nil
}

func sendClientUpdateProgress(client v1.DataSyncService_UploadClient, uploadReq *v1.UploadRequest, pt ProgressTracker,
	dcFileName string,
) error {
	if err := client.Send(uploadReq); err != nil {
		return errors.Wrap(err, "error while sending uploadRequest")
	}
	if err := pt.updateIndexProgressFile(filepath.Join(progressDir, filepath.Base(dcFileName))); err != nil {
		return err
	}
	return nil
}

func sendClient(client v1.DataSyncService_UploadClient, uploadReq *v1.UploadRequest, pt ProgressTracker, dcFileName string) error {
	if err := client.Send(uploadReq); err != nil {
		return errors.Wrap(err, "error while sending uploadRequest")
	}
	return nil
}

func initDataCaptureUpload(ctx context.Context, f *os.File, pt ProgressTracker, dcFileName string, md *v1.UploadMetadata) error {
	progressFilePath := filepath.Join(progressDir, filepath.Base(dcFileName))
	progressIndex, err := pt.getIndexProgressFile(progressFilePath)
	if err != nil {
		return err
	}
	if progressIndex == 0 {
		if err := pt.createProgressFile(progressFilePath, progressIndex); err != nil {
			return err
		}
		return nil
	}

	if err := skipSensordataMessages(ctx, f, progressIndex); err != nil {
		return err
	}
	return nil
}

// Sets the next read/write pointer in a data capture file to the next sensordata message that needs to be uploaded.
func skipSensordataMessages(ctx context.Context, f *os.File, nextMessageIndex int) error {
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	for i := 0; i < nextMessageIndex; i++ {
		if _, err := getNextSensorUploadRequest(ctx, f); err != nil {
			return err
		}
	}
	return nil
}

func getNextFileUploadRequest(ctx context.Context, f *os.File) (*v1.UploadRequest, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:

		// Get the next file data reading from file, check for an error.
		next, err := readNextFileChunk(f)
		if err != nil {
			return nil, err
		}
		// Otherwise, return an UploadRequest and no error.
		return &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_FileContents{
				FileContents: next,
			},
		}, nil
	}
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

func readNextFileChunk(f *os.File) (*v1.FileData, error) {
	byteArr := make([]byte, uploadChunkSize)
	numBytesRead, err := f.Read(byteArr)
	if numBytesRead < uploadChunkSize {
		byteArr = byteArr[:numBytesRead]
	}
	if err != nil {
		return nil, err
	}
	return &v1.FileData{Data: byteArr}, nil
}
