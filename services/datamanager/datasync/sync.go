// Package datasync contains interfaces for syncing data from robots to the app.viam.com cloud.
package datasync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

/**
TODO: We still have the possibility of duplicated data being uploaded if the server sends an ACK (and has thus persisted
      the data), but the client doesn't receive the ACK before shutting down/erroring/etc. In this case the client will
      think the data hasn't been persisted, and will reupload it.
      I think this is solvable, but it may be difficult. As an interim, we can limit the total amount of duplicate data
      risked by lowering the ACK size (since the amount of duplicate data possible == the size of one ACK).
*/

const (
	appAddress = "app.viam.com:443"
)

var (
	// InitialWaitTimeMillis defines the time to wait on the first retried upload attempt.
	InitialWaitTimeMillis = atomic.NewInt32(1000)
	// RetryExponentialFactor defines the factor by which the retry wait time increases.
	RetryExponentialFactor = 2
	maxRetryInterval       = time.Hour
)

// Manager is responsible for enqueuing files in captureDir and uploading them to the cloud.
type Manager interface {
	SyncCaptureFiles(paths []string)
	SyncCaptureQueues(queues []*datacapture.Queue)
	SyncArbitraryFiles(paths []string)
	Close()
}

// syncer is responsible for uploading files in captureDir to the cloud.
type syncer struct {
	partID            string
	conn              rpc.ClientConn
	client            v1.DataSyncServiceClient
	logger            golog.Logger
	backgroundWorkers sync.WaitGroup
	cancelCtx         context.Context
	cancelFunc        func()
	lastModifiedSecs  int

	progressLock *sync.Mutex
	inProgress   map[string]bool
}

// ManagerConstructor is a function for building a Manager.
type ManagerConstructor func(logger golog.Logger, cfg *config.Config, lastModSecs int) (Manager, error)

// NewDefaultManager returns the default Manager that syncs data to app.viam.com.
func NewDefaultManager(logger golog.Logger, cfg *config.Config, lastModSecs int) (Manager, error) {
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
	return NewManager(logger, cfg.Cloud.ID, client, conn, lastModSecs)
}

// NewManager returns a new syncer.
func NewManager(logger golog.Logger, partID string, client v1.DataSyncServiceClient,
	conn rpc.ClientConn, lastModifiedSecs int,
) (Manager, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	ret := syncer{
		conn:              conn,
		client:            client,
		logger:            logger,
		backgroundWorkers: sync.WaitGroup{},
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
		partID:            partID,
		progressLock:      &sync.Mutex{},
		inProgress:        make(map[string]bool),
		lastModifiedSecs:  lastModifiedSecs,
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

// TODO: expose errors somehow
// TODO: sync arbitrary files on ticker too

// SyncCaptureQueues uploads everything in queue until it is closed and emptied.
func (s *syncer) SyncCaptureQueues(queues []*datacapture.Queue) {
	for _, q := range queues {
		s.backgroundWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
			defer s.backgroundWorkers.Done()
			for {
				if err := s.cancelCtx.Err(); err != nil {
					if !errors.Is(err, context.Canceled) {
						s.logger.Error(err)
					}
					return
				}
				s.syncQueue(s.cancelCtx, q)
			}
		})
	}
}

func (s *syncer) SyncCaptureFiles(paths []string) {
	for _, p := range paths {
		newP := p
		s.backgroundWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
			defer s.backgroundWorkers.Done()
			select {
			case <-s.cancelCtx.Done():
				return
			default:
				//nolint:gosec
				f, err := os.Open(newP)
				if err != nil {
					s.logger.Errorw("error opening file", "error", err)
					return
				}

				captureFile, err := datacapture.ReadFile(f)
				if err != nil {
					s.logger.Errorw("error reading capture file", "error", err)
					err := f.Close()
					if err != nil {
						s.logger.Errorw("error closing file", "error", err)
					}
					return
				}
				s.syncDataCaptureFile(captureFile)
			}
		})
	}
}

func (s *syncer) SyncArbitraryFiles(dirs []string) {
	fmt.Println("reached SyncArbitraryFiles in sync")
	fmt.Println(fmt.Sprintf("uploading files in %v", dirs))
	var paths []string
	for _, dir := range dirs {
		paths = append(paths, getAllFilesToSync(dir, s.lastModifiedSecs)...)
	}
	fmt.Println(fmt.Sprintf("uploading paths: %v", paths))

	for _, p := range paths {
		newP := p
		s.backgroundWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
			fmt.Println(fmt.Sprintf("uploading %s", newP))
			defer s.backgroundWorkers.Done()
			if !s.markInProgress(newP) {
				return
			}
			defer s.unmarkInProgress(newP)

			f, err := os.Open(newP)
			if err != nil {
				s.logger.Errorw("error opening file", "error", err)
				return
			}
			s.syncArbitraryFile(f)
		})
	}
}

func (s *syncer) syncQueue(ctx context.Context, q *datacapture.Queue) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			next, err := q.Pop()
			if err != nil {
				s.logger.Error(err)
				return
			}
			if next == nil && q.IsClosed() {
				return
			}

			if next == nil {
				break
			}
			s.syncDataCaptureFile(next)
		}
	}
}

func (s *syncer) syncDataCaptureFile(f *datacapture.File) {
	if !s.markInProgress(f.GetPath()) {
		return
	}

	uploadErr := exponentialRetry(
		s.cancelCtx,
		func(ctx context.Context) error {
			return uploadDataCaptureFile(ctx, s.client, f, s.partID)
		},
		s.logger,
	)
	if uploadErr != nil {
		s.logger.Error(uploadErr)
		err := f.Close()
		if err != nil {
			s.logger.Errorw("error closing file", "error", err)
		}
		return
	}
	if err := f.Delete(); err != nil {
		s.logger.Error(err)
		err := f.Close()
		if err != nil {
			s.logger.Errorw("error closing file", "error", err)
		}
		return
	}
	s.unmarkInProgress(f.GetPath())
}

func (s *syncer) syncArbitraryFile(f *os.File) {
	uploadErr := exponentialRetry(
		s.cancelCtx,
		func(ctx context.Context) error {
			return uploadArbitraryFile(ctx, s.client, f, s.partID)
		},
		s.logger,
	)
	if uploadErr != nil {
		s.logger.Error(uploadErr)
		err := f.Close()
		if err != nil {
			s.logger.Errorw("error closing file", "error", err)
		}
		return
	}
	if err := os.Remove(f.Name()); err != nil {
		s.logger.Error(fmt.Sprintf("error deleting file %s", f.Name()), "error", err)
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
	s.inProgress[path] = false
	return
}

// exponentialRetry calls fn, logs any errors, and retries with exponentially increasing waits from initialWait to a
// maximum of maxRetryInterval.
func exponentialRetry(cancelCtx context.Context, fn func(cancelCtx context.Context) error, log golog.Logger) error {
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
		return time.Millisecond * time.Duration(InitialWaitTimeMillis.Load())
	}
	nextWait := lastWait * time.Duration(RetryExponentialFactor)
	if nextWait > maxRetryInterval {
		return maxRetryInterval
	}
	return nextWait
}

func getAllFilesToSync(dir string, lastModifiedSecs int) []string {
	fmt.Println("getting all files to sync")
	var filePaths []string

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		fmt.Println(path)
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// If a file was modified within the past waitAfterLastModifiedSecs seconds, do not sync it (data
		// may still be being written).
		if diff := time.Now().Sub(info.ModTime()); diff < (time.Duration(lastModifiedSecs) * time.Second) {
			return nil
		}
		filePaths = append(filePaths, path)
		return nil
	})
	return filePaths
}
