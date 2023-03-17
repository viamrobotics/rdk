// Package datasync contains interfaces for syncing data from robots to the app.viam.com cloud.
package datasync

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager/datacapture"
	rdkutils "go.viam.com/rdk/utils"
)

var (
	// InitialWaitTimeMillis defines the time to wait on the first retried upload attempt.
	InitialWaitTimeMillis = atomic.NewInt32(1000)
	// RetryExponentialFactor defines the factor by which the retry wait time increases.
	RetryExponentialFactor = atomic.NewInt32(2)
	maxRetryInterval       = time.Hour
)

// TODO: Goal: don't log cancel if cancelled because Manager was closed

// Manager is responsible for enqueuing files in captureDir and uploading them to the cloud.
type Manager interface {
	SyncDirectory(dir string)
	Close()
}

// syncer is responsible for uploading files in captureDir to the cloud.
type syncer struct {
	partID             string
	conn               rpc.ClientConn
	client             v1.DataSyncServiceClient
	logger             golog.Logger
	backgroundWorkers  sync.WaitGroup
	cancelCtx          context.Context
	cancelFunc         func()
	lastModifiedMillis int

	progressLock *sync.Mutex
	inProgress   map[string]bool

	syncErrs   chan error
	closed     *atomic.Bool
	logRoutine sync.WaitGroup
}

// ManagerConstructor is a function for building a Manager.
type ManagerConstructor func(logger golog.Logger, cfg *config.Config, lastModMillis int) (Manager, error)

// NewDefaultManager returns the default Manager that syncs data to app.viam.com.
func NewDefaultManager(logger golog.Logger, cfg *config.Config, lastModMillis int) (Manager, error) {
	if cfg.Cloud == nil || cfg.Cloud.AppAddress == "" {
		logger.Debug("Using no-op sync manager when Cloud config is not available")
		return NewNoopManager(), nil
	}

	tlsConfig := config.NewTLSConfig(cfg).Config
	rpcOpts := []rpc.DialOption{
		rpc.WithTLSConfig(tlsConfig),
		rpc.WithEntityCredentials(
			cfg.Cloud.ID,
			rpc.Credentials{
				Type:    rdkutils.CredentialsTypeRobotSecret,
				Payload: cfg.Cloud.Secret,
			}),
	}

	appURLParsed, err := url.Parse(cfg.Cloud.AppAddress)
	if err != nil {
		return nil, err
	}
	conn, err := NewConnection(logger, appURLParsed.Host, rpcOpts)
	if err != nil {
		return nil, err
	}
	client := NewClient(conn)
	return NewManager(logger, cfg.Cloud.ID, client, conn, lastModMillis)
}

// NewManager returns a new syncer.
func NewManager(logger golog.Logger, partID string, client v1.DataSyncServiceClient,
	conn rpc.ClientConn, lastModifiedMillis int,
) (Manager, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	ret := syncer{
		partID:             partID,
		conn:               conn,
		client:             client,
		logger:             logger,
		backgroundWorkers:  sync.WaitGroup{},
		cancelCtx:          cancelCtx,
		cancelFunc:         cancelFunc,
		lastModifiedMillis: lastModifiedMillis,
		progressLock:       &sync.Mutex{},
		inProgress:         make(map[string]bool),

		// TODO: channel size?
		syncErrs:   make(chan error, 100),
		closed:     &atomic.Bool{},
		logRoutine: sync.WaitGroup{},
	}
	ret.logRoutine.Add(1)
	goutils.PanicCapturingGo(func() {
		defer ret.logRoutine.Done()
		ret.log()
	})
	return &ret, nil
}

// Close closes all resources (goroutines) associated with s.
func (s *syncer) Close() {
	s.closed.Store(true)

	s.cancelFunc()
	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			s.logger.Errorw("error closing datasync server connection", "error", err)
		}
	}
	// TODO: this is waiting for the syncErrs channel to be drained, but it's not being drained
	s.backgroundWorkers.Wait()
	close(s.syncErrs)
	s.logRoutine.Wait()
}

func (s *syncer) SyncDirectory(dir string) {
	paths := getAllFilesToSync(dir, s.lastModifiedMillis)
	for _, p := range paths {
		newP := p
		s.backgroundWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
			defer s.backgroundWorkers.Done()
			select {
			case <-s.cancelCtx.Done():
				return
			default:
				if !s.markInProgress(newP) {
					return
				}
				//nolint:gosec
				f, err := os.Open(newP)
				if err != nil {
					// Don't log if the file does not exist, because that means it was successfully synced and deleted
					// in between paths being built and this executing.
					if !errors.Is(err, os.ErrNotExist) {
						s.syncErrs <- errors.Wrap(err, "error opening file")
					}
					return
				}

				if datacapture.IsDataCaptureFile(f) {
					captureFile, err := datacapture.ReadFile(f)
					if err != nil {
						s.syncErrs <- errors.Wrap(err, "error reading data capture file")
						err := f.Close()
						if err != nil {
							s.syncErrs <- errors.Wrap(err, "error closing data capture file")
						}
						return
					}
					s.syncDataCaptureFile(captureFile)
				} else {
					s.syncArbitraryFile(f)
				}
				s.unmarkInProgress(newP)
			}
		})
	}
}

func (s *syncer) syncDataCaptureFile(f *datacapture.File) {
	uploadErr := exponentialRetry(
		s.cancelCtx,
		func(ctx context.Context) error {
			err := uploadDataCaptureFile(ctx, s.client, f, s.partID)
			if err != nil {
				// TODO: All the routines are blocking here
				s.syncErrs <- errors.Wrap(err, fmt.Sprintf("error uploading file %s", f.GetPath()))
			}
			return err
		},
	)
	if uploadErr != nil {
		err := f.Close()
		if err != nil {
			s.syncErrs <- errors.Wrap(err, "error closing data capture file")
		}
		return
	}
	if err := f.Delete(); err != nil {
		s.syncErrs <- errors.Wrap(err, "error deleting data capture file")
		return
	}
}

func (s *syncer) syncArbitraryFile(f *os.File) {
	uploadErr := exponentialRetry(
		s.cancelCtx,
		func(ctx context.Context) error {
			err := uploadArbitraryFile(ctx, s.client, f, s.partID)
			if err != nil {
				s.syncErrs <- errors.Wrap(err, fmt.Sprintf("error uploading file %s", f.Name()))
			}
			return err
		})
	if uploadErr != nil {
		err := f.Close()
		if err != nil {
			s.syncErrs <- errors.Wrap(err, "error closing data capture file")
		}
		return
	}
	if err := os.Remove(f.Name()); err != nil {
		s.syncErrs <- errors.Wrap(err, fmt.Sprintf("error deleting file %s", f.Name()))
		return
	}
}

// markInProgress marks path as in progress in s.inProgress. It returns true if it changed the progress status,
// or false if the path was already in progress.
func (s *syncer) markInProgress(path string) bool {
	s.progressLock.Lock()
	defer s.progressLock.Unlock()
	if s.inProgress[path] {
		return false
	}
	s.inProgress[path] = true
	return true
}

func (s *syncer) unmarkInProgress(path string) {
	s.progressLock.Lock()
	defer s.progressLock.Unlock()
	delete(s.inProgress, path)
}

func (s *syncer) log() {
	for err := range s.syncErrs {
		if s.closed.Load() {
			// Don't log context cancellation errors if the collector has already been closed. This means the collector
			// cancelled the context, and the context cancellation error is expected.
			if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), context.Canceled.Error()) {
				continue
			}
		}
		s.logger.Error(err)
	}
	return
}

// exponentialRetry calls fn and retries with exponentially increasing waits from initialWait to a
// maximum of maxRetryInterval.
func exponentialRetry(cancelCtx context.Context, fn func(cancelCtx context.Context) error) error {
	// Only create a ticker and enter the retry loop if we actually need to retry.
	var err error
	if err = fn(cancelCtx); err == nil {
		return nil
	}
	// Don't retry non-retryable errors.
	s := status.Convert(err)
	if s.Code() == codes.InvalidArgument {
		return err
	}

	// First call failed, so begin exponentialRetry with a factor of RetryExponentialFactor
	nextWait := time.Millisecond * time.Duration(InitialWaitTimeMillis.Load())
	ticker := time.NewTicker(nextWait)
	for {
		if err := cancelCtx.Err(); err != nil {
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
		return time.Millisecond * time.Duration(InitialWaitTimeMillis.Load())
	}
	nextWait := lastWait * time.Duration(RetryExponentialFactor.Load())
	if nextWait > maxRetryInterval {
		return maxRetryInterval
	}
	return nextWait
}

// nolint
func getAllFilesToSync(dir string, lastModifiedMillis int) []string {
	var filePaths []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// If a file was modified within the past waitAfterLastModifiedSecs seconds, do not sync it (data
		// may still be being written).
		timeSinceMod := time.Since(info.ModTime())
		if timeSinceMod > (time.Duration(lastModifiedMillis)*time.Millisecond) || filepath.Ext(path) == datacapture.FileExt {
			filePaths = append(filePaths, path)
		}
		return nil
	})
	return filePaths
}
