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
	progressTracker   progressTracker
	uploadFunc        uploadFunc
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

type (
	getNextUploadRequestFunc func(context.Context, *os.File) (*v1.UploadRequest, error)
	processUploadRequestFunc func(v1.DataSyncService_UploadClient, *v1.UploadRequest, progressTracker, string) error
	uploadFunc               func(context.Context, v1.DataSyncService_UploadClient, string, string) error
)

// TODO DATA-206: instantiate a client
// newSyncer returns a new syncer. If a nil uploadFunc is passed, the default viamUpload is used.
func newSyncer(logger golog.Logger, uploadFunc uploadFunc, partID string) *syncer {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	ret := syncer{
		logger: logger,
		progressTracker: progressTracker{
			lock:        &sync.Mutex{},
			m:           make(map[string]struct{}),
			progressDir: filepath.Join(viamCaptureDotDir, ".progress/"),
		},
		backgroundWorkers: sync.WaitGroup{},
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
		partID:            partID,
	}
	if uploadFunc == nil {
		uploadFunc = ret.uploadFile
	}
	ret.uploadFunc = uploadFunc
	if err := ret.progressTracker.initProgressDir(); err != nil {
		logger.Warn("couldn't initialize progress tracking directory")
	}
	return &ret
}

// Close closes all resources (goroutines) associated with s.
func (s *syncer) Close() {
	s.cancelFunc()
	s.backgroundWorkers.Wait()
}

func (s *syncer) upload(ctx context.Context, path string) {
	if s.progressTracker.inProgress(path) {
		return
	}

	s.progressTracker.mark(path)
	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer s.backgroundWorkers.Done()
		uploadErr := exponentialRetry(
			ctx,
			func(ctx context.Context) error { return s.uploadFunc(ctx, s.client, path, s.partID) },
			s.logger,
		)
		if uploadErr != nil {
			return
		}

		// Delete the file and indicate that the upload is done.
		if err := os.Remove(path); err != nil {
			s.logger.Errorw("error while deleting file", "error", err)
		} else {
			s.progressTracker.unmark(path)
			if err := s.progressTracker.deleteProgressFile(filepath.Join(s.progressTracker.progressDir,
				filepath.Base(path))); err !=
				nil {
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

func getMetadataUploadRequest(f *os.File, partID string) (*v1.UploadRequest, error) {
	var md *v1.UploadMetadata
	if isDataCaptureFile(f) {
		captureMD, err := readDataCaptureMetadata(f)
		if err != nil {
			return &v1.UploadRequest{}, err
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
	return &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: md,
		},
	}, nil
}

func (s *syncer) uploadFile(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partID string) error {
	//nolint:gosec
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "error while opening file %s", path)
	}

	// Resets file pointer to ensure we are reading from beginning of file.
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}

	req, err := getMetadataUploadRequest(f, partID)
	if err != nil {
		return err
	}

	var getNextUploadRequest getNextUploadRequestFunc
	var processUploadRequest processUploadRequestFunc
	switch req.GetMetadata().GetType() {
	case v1.DataType_DATA_TYPE_BINARY_SENSOR, v1.DataType_DATA_TYPE_TABULAR_SENSOR:
		err = initDataCaptureUpload(ctx, f, s.progressTracker, f.Name(), req.GetMetadata())
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		getNextUploadRequest = getNextSensorUploadRequest
		processUploadRequest = sendReqAndUpdateProgress
	case v1.DataType_DATA_TYPE_FILE:
		getNextUploadRequest = getNextFileUploadRequest
		processUploadRequest = sendReq
	case v1.DataType_DATA_TYPE_UNSPECIFIED:
		return errors.New("no data type specified in upload metadata")
	default:
		return errors.New("no data type specified in upload metadata")
	}

	if err := client.Send(req); err != nil {
		return errors.Wrap(err, "error while sending upload metadata")
	}

	// Loop until there is no more content to be read from file.
	for {
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextUploadRequest(ctx, f)
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

		if err = processUploadRequest(client, uploadReq, s.progressTracker, f.Name()); err != nil {
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

func sendReqAndUpdateProgress(client v1.DataSyncService_UploadClient, uploadReq *v1.UploadRequest, pt progressTracker,
	dcFileName string,
) error {
	if err := client.Send(uploadReq); err != nil {
		return errors.Wrap(err, "error while sending uploadRequest")
	}
	if err := pt.updateProgressFileIndex(filepath.Join(pt.progressDir, filepath.Base(dcFileName))); err != nil {
		return err
	}
	return nil
}

func sendReq(client v1.DataSyncService_UploadClient, uploadReq *v1.UploadRequest, pt progressTracker, dcFileName string) error {
	if err := client.Send(uploadReq); err != nil {
		return errors.Wrap(err, "error while sending uploadRequest")
	}
	return nil
}

func initDataCaptureUpload(ctx context.Context, f *os.File, pt progressTracker, dcFileName string, md *v1.UploadMetadata) error {
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
	finfo, err := f.Stat()
	if err != nil {
		return err
	}
	if currentOffset == finfo.Size() {
		return io.EOF
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
