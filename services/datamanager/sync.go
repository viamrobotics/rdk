package datamanager

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	goutils "go.viam.com/utils"
)

var (
	initialWaitTime        = time.Second
	retryExponentialFactor = 2
	maxRetryInterval       = time.Hour
	// Chunk size set at 32 kiB, this is 32768 Bytes.
	uploadChunkSize       = 32768
	hardCodePartName      = "TODO [DATA-164]"
	hardCodeMethodName    = "TODO [DATA-164]"
	hardCodeComponentName = "TODO [DATA-164]"
)

func emptyReadingErr(fileName string) error {
	return errors.Errorf("%s is empty", fileName)
}

// syncer is responsible for enqueuing files in captureDir and uploading them to the cloud.
type syncManager interface {
	Sync(paths []string)
	Close()
}

// syncer is responsible for uploading files in captureDir to the cloud.
type syncer struct {
	logger            golog.Logger
	progressTracker   progressTracker
	uploadFn          uploadFn
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

type uploadFn func(ctx context.Context, client v1.DataSyncService_UploadClient, path string) error

// newSyncer returns a new syncer.
func newSyncer(logger golog.Logger, uploadFunc uploadFn) *syncer {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	if uploadFunc == nil {
		uploadFunc = viamUpload
	}
	ret := syncer{
		logger: logger,
		progressTracker: progressTracker{
			lock: &sync.Mutex{},
			m:    make(map[string]struct{}),
		},
		backgroundWorkers: sync.WaitGroup{},
		uploadFn:          uploadFunc,
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
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
			func(ctx context.Context) error { return s.uploadFn(ctx, nil, path) },
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
		}
	})
}

func (s *syncer) Sync(paths []string) {
	for _, p := range paths {
		s.upload(s.cancelCtx, p)
	}
}

type progressTracker struct {
	lock *sync.Mutex
	m    map[string]struct{}
}

func (p *progressTracker) inProgress(k string) bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, ok := p.m[k]
	return ok
}

func (p *progressTracker) mark(k string) {
	p.lock.Lock()
	p.m[k] = struct{}{}
	p.lock.Unlock()
}

//nolint:unused
func (p *progressTracker) unmark(k string) {
	p.lock.Lock()
	delete(p.m, k)
	p.lock.Unlock()
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

func getDataTypeFromLeadingMessage(f *os.File) (v1.DataType, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return v1.DataType_DATA_TYPE_UNSPECIFIED, err
	}

	// Read the file as if it is SensorData, and if the error is EOF (end of file) that means the filetype is
	// arbitrary because no protobuf-generated structs (for SensorData) were able to be read (and thus the file has no
	// readable data). If the error is anything other than EOF, return the data type as UNSPECIFIED with the
	// corresponding error.
	sensorData, err := readNextSensorData(f)

	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return v1.DataType_DATA_TYPE_FILE, nil
	}

	// If sensorData contains a struct it's tabular data; if it contains binary it's binary data; if it contains
	// neither it is invalid.
	if s := sensorData.GetStruct(); s != nil {
		return v1.DataType_DATA_TYPE_TABULAR_SENSOR, nil
	}
	if b := sensorData.GetBinary(); b != nil {
		return v1.DataType_DATA_TYPE_BINARY_SENSOR, nil
	}
	return v1.DataType_DATA_TYPE_UNSPECIFIED, errors.New("sensordata data type not specified")
}

func viamUpload(ctx context.Context, client v1.DataSyncService_UploadClient, path string) error {
	//nolint
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "error while opening file %s", path)
	}
	// Resets file pointer; if you ever want to go back to the start of a file, need to call this
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}

	// Parse filepath to get metadata about the file which we will be reading from.
	// TODO: construct metadata gRPC message that contains PartName, ComponentName, MethodName

	// dataType represents the protobuf DataType value describing the file to be uploaded.
	dataType, err := getDataTypeFromLeadingMessage(f)
	if err != nil {
		return errors.Wrap(err, "error while getting metadata data type")
	}

	// METADATA FIELDS FOR CONSTRUCTION BELOW:
	// PartName: TODO [DATA-164]
	// ComponentName: Above
	// MethodName: TODO [DATA-164]
	// Type: Above
	// FileName: Above

	// Actually construct the Metadata
	md := &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			// TODO: Figure out best way to pass these in.
			Metadata: &v1.UploadMetadata{
				PartName:      hardCodePartName,
				ComponentName: hardCodeComponentName,
				MethodName:    hardCodeMethodName,
				Type:          dataType,
				FileName:      filepath.Base(f.Name()),
			},
		},
	}
	if err := client.Send(md); err != nil {
		return errors.Wrap(err, "error while sending upload metadata")
	}

	var getNextRequest func(context.Context, *os.File) (*v1.UploadRequest, error)

	switch md.GetMetadata().GetType() {
	case v1.DataType_DATA_TYPE_BINARY_SENSOR, v1.DataType_DATA_TYPE_TABULAR_SENSOR:
		getNextRequest = getNextSensorUploadRequest
	case v1.DataType_DATA_TYPE_FILE:
		getNextRequest = getNextFileUploadRequest
	case v1.DataType_DATA_TYPE_UNSPECIFIED:
		return errors.New("no data type specified in upload metadata")
	default:
		return errors.New("no data type specified in upload metadata")
	}

	// Reset file pointer to ensure we are reading from beginning of file.
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}
	// Loop until there is no more content to be read from file.
	for {
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextRequest(ctx, f)
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
		if err := client.Send(uploadReq); err != nil {
			return errors.Wrap(err, "error while sending uploadRequest")
		}
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Close stream and receive response.
	if _, err := client.CloseAndRecv(); err != nil {
		return errors.Wrap(err, "error when closing the stream and receiving the response from "+
			"sync service backend")
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

func readNextSensorData(f *os.File) (*v1.SensorData, error) {
	if _, err := f.Seek(0, 1); err != nil {
		return nil, err
	}
	r := &v1.SensorData{}
	if _, err := pbutil.ReadDelimited(f, r); err != nil {
		return nil, err
	}

	// Ensure we construct and return a SensorData value for tabular data when the tabular data's fields and
	// corresponding entries are not nil. Otherwise, return io.EOF error and nil.
	if r.GetBinary() == nil {
		if r.GetStruct() == nil {
			return r, emptyReadingErr(filepath.Base(f.Name()))
		}
		return r, nil
	}
	return r, nil
}

func readNextFileChunk(f *os.File) (*v1.FileData, error) {
	if _, err := f.Seek(0, 1); err != nil {
		return nil, err
	}
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
