package datamanager

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	// Buffer size set at 32 kiB, this is 32768 Bytes.
	bufferSize         = 32768
	hardCodePartName   = "TODO [DATA-164]"
	hardCodeMethodName = "TODO [DATA-164]"
)

type emptyFileError struct{}

func (m *emptyFileError) Error() string {
	return "this file is empty!"
}

// Syncer is responsible for enqueuing files in captureDir and uploading them to the cloud.
type syncManager interface {
	Start()
	Enqueue(filesToQueue []string) error
	Close()
}

// TODO: replace uploadFn with some Uploader interface with Upload/Close methods syncer is responsible for
// enqueuing files in captureDir and uploading them to the cloud.
type syncer struct {
	captureDir        string
	syncQueue         string
	logger            golog.Logger
	queueWaitTime     time.Duration
	progressTracker   progressTracker
	uploadFn          uploadFn
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

type uploadFn func(ctx context.Context, client v1.DataSyncService_UploadClient, path string) error

// newSyncer returns a new syncer.
func newSyncer(queuePath string, logger golog.Logger, captureDir string) *syncer {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	ret := syncer{
		syncQueue:     queuePath,
		logger:        logger,
		captureDir:    captureDir,
		queueWaitTime: time.Minute,
		progressTracker: progressTracker{
			lock: &sync.Mutex{},
			m:    make(map[string]struct{}),
		},
		backgroundWorkers: sync.WaitGroup{},
		uploadFn:          viamUpload,
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
	}

	return &ret
}

// Enqueue moves files that are no longer being written to from captureDir to SyncQueuePath.
func (s *syncer) Enqueue(filesToQueue []string) error {
	for _, filePath := range filesToQueue {
		subPath, err := s.getPathUnderCaptureDir(filePath)
		if err != nil {
			return errors.Errorf("could not get path under capture directory: %v", err)
		}

		if err := os.MkdirAll(filepath.Dir(path.Join(s.syncQueue, subPath)), 0o700); err != nil {
			return errors.Errorf("failed create directories under sync enqueue: %v", err)
		}

		if err := os.Rename(filePath, path.Join(s.syncQueue, subPath)); err != nil {
			return errors.Errorf("failed to move file to sync enqueue: %v", err)
		}
	}
	return nil
}

// Start queues any files already in captureDir that haven't been modified in s.queueWaitTime time, and kicks off a
// goroutine to constantly upload files in the queue.
func (s *syncer) Start() {
	// First, move any files in captureDir to queue.
	if err := filepath.WalkDir(s.captureDir, s.queueFile); err != nil {
		s.logger.Errorf("failed to move files to sync queue: %v", err)
	}

	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		ticker := time.NewTicker(time.Millisecond * 500)
		defer ticker.Stop()
		defer s.backgroundWorkers.Done()
		for {
			if err := s.cancelCtx.Err(); err != nil {
				if !errors.Is(err, context.Canceled) {
					s.logger.Errorw("sync context closed unexpectedly", "error", err)
				}
				return
			}
			select {
			case <-s.cancelCtx.Done():
				return
			case <-ticker.C:
				if err := filepath.WalkDir(s.syncQueue, s.upload); err != nil {
					s.logger.Errorf("failed to upload queued file: %v", err)
				}
			}
		}
	})
}

// queueFile is an fs.WalkDirFunc that moves matching files to s.syncQueue.
func (s *syncer) queueFile(filePath string, di fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if di.IsDir() {
		return nil
	}

	fileInfo, err := di.Info()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to get file info for filepath %s", filePath))
	}

	// If it's been written to in the last s.queueWaitTime, it's still active and shouldn't be queued.
	if time.Since(fileInfo.ModTime()) < s.queueWaitTime {
		return nil
	}

	subPath, err := s.getPathUnderCaptureDir(filePath)
	if err != nil {
		return errors.Wrap(err, "could not get path under capture directory")
	}

	if err = os.MkdirAll(filepath.Dir(path.Join(s.syncQueue, subPath)), 0o700); err != nil {
		return errors.Wrap(err, "failed create directories under sync enqueue")
	}

	if err := os.Rename(filePath, path.Join(s.syncQueue, subPath)); err != nil {
		return errors.Wrap(err, "failed to move file to sync enqueue")
	}
	return nil
}

func (s *syncer) getPathUnderCaptureDir(filePath string) (string, error) {
	if idx := strings.Index(filePath, s.captureDir); idx != -1 {
		return filePath[idx+len(s.captureDir):], nil
	}
	return "", errors.Errorf("file path %s is not under capture directory %s", filePath, s.captureDir)
}

// Close closes all resources (goroutines) associated with s.
func (s *syncer) Close() {
	s.cancelFunc()
	s.backgroundWorkers.Wait()
}

// upload is an fs.WalkDirFunc that uploads files to Viam cloud storage.
func (s *syncer) upload(path string, di fs.DirEntry, err error) error {
	if err != nil {
		s.logger.Errorw("failed to upload queued file", "error", err)

		return nil
	}

	if di.IsDir() {
		return nil
	}

	if s.progressTracker.inProgress(path) {
		return nil
	}

	s.progressTracker.mark(path)
	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer s.backgroundWorkers.Done()
		exponentialRetry(
			s.cancelCtx,
			// TODO: figure out how to build client, and make it a field of s.
			func(ctx context.Context) error { return s.uploadFn(ctx, nil, path) },
			s.logger,
		)
	})
	// TODO: If upload completed successfully, unmark in-progress and delete file.
	return nil
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
func exponentialRetry(ctx context.Context, fn func(ctx context.Context) error, log golog.Logger) {
	// Only create a ticker and enter the retry loop if we actually need to retry.
	if err := fn(ctx); err == nil {
		return
	}

	// First call failed, so begin exponentialRetry with a factor of retryExponentialFactor
	nextWait := initialWaitTime
	ticker := time.NewTicker(nextWait)
	for {
		if err := ctx.Err(); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Errorw("context closed unexpectedly", "error", err)
			}
			return
		}

		select {
		// If cancelled, return nil.
		case <-ctx.Done():
			ticker.Stop()
			return
		// Otherwise, try again after nextWait.
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				// If error, retry with a new nextWait.
				log.Errorw("error while uploading file", "error", err)
				ticker.Stop()
				nextWait = getNextWait(nextWait)
				ticker = time.NewTicker(nextWait)
				continue
			}
			// If no error, return.
			ticker.Stop()
			return
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

	// (UNCOMMENT BELOW) syncQueuePath is directory where files are queued while waiting to be synced to cloud.
	// syncQueuePath := filepath.Clean(f.Name()[:(strings.Index(f.Name(), ".viam") + 5)])

	// remainingPathContent is a slice of strings that includes subtypeName & componentName (in that order).
	remainingPathContent := strings.Split(f.Name()[len(viamCaptureDotDir):], "/")
	componentName := remainingPathContent[1]

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
				ComponentName: componentName,
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
		if errors.Is(err, &emptyFileError{}) {
			continue
		}
		// If there is any other error, return it.
		if err != nil {
			return err
		}
		// Finally, send the UploadRequest to the client.
		if err := client.Send(uploadReq); err != nil {
			return errors.Wrap(err, "error while sending fileData")
		}
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Close stream and receive response.
	if _, err := client.CloseAndRecv(); err != nil {
		return errors.Wrap(err, "error when closing the stream and receiving the response from sync service backend")
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
			return r, &emptyFileError{}
		}
		return r, nil
	}
	return r, nil
}

func readNextFileChunk(f *os.File) (*v1.FileData, error) {
	if _, err := f.Seek(0, 1); err != nil {
		return nil, err
	}
	byteArr := make([]byte, bufferSize)
	numBytesRead, err := f.Read(byteArr)
	if numBytesRead < bufferSize {
		byteArr = byteArr[:numBytesRead]
	}
	if err != nil {
		return nil, err
	}
	return &v1.FileData{Data: byteArr}, nil
}
