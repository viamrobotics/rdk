// Package datasync contains interfaces for syncing data from robots to the app.viam.com cloud.
package datasync

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager/datacapture"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	appAddress = "app.viam.com:443"
)

var (
	initialWaitTime        = time.Second
	retryExponentialFactor = 2
	maxRetryInterval       = time.Hour
	// Chunk size set at 32 kiB, this is 32768 Bytes.
	uploadChunkSize = 32768
)

// Manager is responsible for enqueuing files in captureDir and uploading them to the cloud.
type Manager interface {
	Sync(paths []string)
	Close()
}

// syncer is responsible for uploading files in captureDir to the cloud.
type syncer struct {
	partID            string
	conn              rpc.ClientConn
	client            v1.DataSyncServiceClient
	logger            golog.Logger
	progressTracker   progressTracker
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
}

// ManagerConstructor is a function for building a Manager.
type ManagerConstructor func(logger golog.Logger, cfg *config.Config) (Manager, error)

// NewDefaultManager returns the default Manager that syncs data to app.viam.com.
func NewDefaultManager(logger golog.Logger, cfg *config.Config) (Manager, error) {
	tlsConfig := config.NewTLSConfig(cfg).Config
	cloudConfig := cfg.Cloud
	rpcOpts := []rpc.DialOption{
		rpc.WithTLSConfig(tlsConfig),
		rpc.WithEntityCredentials(
			cloudConfig.ID,
			rpc.Credentials{
				Type:    rdkutils.CredentialsTypeRobotSecret,
				Payload: cloudConfig.Secret,
			}),
	}

	conn, err := NewConnection(logger, appAddress, rpcOpts)
	if err != nil {
		return nil, err
	}
	client := NewClient(conn)
	return NewManager(logger, cfg.Cloud.ID, client, conn)
}

// NewManager returns a new syncer.
func NewManager(logger golog.Logger, partID string, client v1.DataSyncServiceClient,
	conn rpc.ClientConn,
) (Manager, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	ret := syncer{
		conn:   conn,
		client: client,
		logger: logger,
		progressTracker: progressTracker{
			lock:        &sync.Mutex{},
			m:           make(map[string]struct{}),
			progressDir: viamProgressDotDir,
		},
		backgroundWorkers: sync.WaitGroup{},
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
		partID:            partID,
	}
	if err := ret.progressTracker.initProgressDir(); err != nil {
		return nil, errors.Wrap(err, "couldn't initialize progress tracking directory")
	}
	return &ret, nil
}

// Close closes all resources (goroutines) associated with s.
func (s *syncer) Close() {
	s.cancelFunc()
	s.backgroundWorkers.Wait()
	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			s.logger.Errorw("error closing datasync server connection", "error", err)
		}
	}
}

func (s *syncer) upload(ctx context.Context, path string) {
	if s.progressTracker.inProgress(path) {
		return
	}

	s.progressTracker.mark(path)
	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer s.backgroundWorkers.Done()
		//nolint:gosec
		f, err := os.Open(path)
		if err != nil {
			s.logger.Errorw("error opening file", "error", err)
			return
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				s.logger.Errorw("error closing file", "error", err)
			}
		}(f)

		uploadErr := exponentialRetry(
			ctx,
			func(ctx context.Context) error { return s.uploadFile(ctx, s.client, f, s.partID) },
			s.logger,
		)
		if uploadErr != nil {
			s.logger.Error(uploadErr)
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

func getMetadata(f *os.File, partID string) (*v1.UploadMetadata, error) {
	var md *v1.UploadMetadata
	if datacapture.IsDataCaptureFile(f) {
		captureMD, err := datacapture.ReadDataCaptureMetadata(f)
		if err != nil {
			return nil, err
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
	return md, nil
}

func (s *syncer) uploadFile(ctx context.Context, client v1.DataSyncServiceClient, f *os.File, partID string) error {
	// Resets file pointer to ensure we are reading from beginning of file.
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}

	md, err := getMetadata(f, partID)
	if err != nil {
		return err
	}

	switch md.GetType() {
	case v1.DataType_DATA_TYPE_BINARY_SENSOR, v1.DataType_DATA_TYPE_TABULAR_SENSOR:
		return uploadDataCaptureFile(ctx, s.progressTracker, client, md, f)
	case v1.DataType_DATA_TYPE_FILE:
		return uploadArbitraryFile(ctx, client, md, f)
	case v1.DataType_DATA_TYPE_UNSPECIFIED:
		return errors.New("no data type specified in upload metadata")
	default:
		return errors.New("no data type specified in upload metadata")
	}
}
