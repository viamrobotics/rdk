// Package datasync contains interfaces for syncing data from robots to the app.viam.com cloud.
package datasync

import (
	"context"
	"fmt"
	"os"
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
	InitialWaitTimeMillis  = atomic.NewInt32(1000)
	RetryExponentialFactor = 2
	maxRetryInterval       = time.Hour
	PollWaitTime           = time.Second
)

// Manager is responsible for enqueuing files in captureDir and uploading them to the cloud.
type Manager interface {
	SyncCaptureFiles(paths []string)
	SyncCaptureQueues(queues []*datacapture.Queue)
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
		conn:              conn,
		client:            client,
		logger:            logger,
		backgroundWorkers: sync.WaitGroup{},
		cancelCtx:         cancelCtx,
		cancelFunc:        cancelFunc,
		partID:            partID,
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
// Sync uploads everything in queue until it is closed and emptied.
func (s *syncer) SyncCaptureQueues(queues []*datacapture.Queue) {
	for _, q := range queues {
		s.syncQueue(s.cancelCtx, q)
	}
}

func (s *syncer) syncQueue(ctx context.Context, q *datacapture.Queue) {
	s.backgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer s.backgroundWorkers.Done()
		// TODO: make respect cancellation
		for {
			select {
			case <-ctx.Done():
				return
			default:
				next, err := q.Pop()
				if next == nil && q.IsClosed() {
					return
				}

				// TODO: handle other error
				if err != nil {
					s.logger.Error(err)
					return
				}

				if next == nil {
					// TODO: better way to wait than just sleep?
					time.Sleep(PollWaitTime)
					continue
				}

				uploadErr := exponentialRetry(
					ctx,
					func(ctx context.Context) error {
						return s.syncFile(ctx, s.client, next, s.partID)
					},
					s.logger,
				)
				fmt.Println("exponential retry returned")
				if uploadErr != nil {
					fmt.Println("got upload err")
					s.logger.Error(uploadErr)
					return
				}
				fmt.Println("reached delete")
				if err := next.Delete(); err != nil {
					s.logger.Error(err)
					return
				}
			}
		}
	})
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
					s.logger.Error(err)
					err := f.Close()
					if err != nil {
						s.logger.Errorw("error closing file", "error", err)
					}
					return
				}
				uploadErr := exponentialRetry(
					s.cancelCtx,
					func(ctx context.Context) error {
						return s.syncFile(ctx, s.client, captureFile, s.partID)
					},
					s.logger,
				)
				fmt.Println("exponential retry returned")
				if uploadErr != nil {
					fmt.Println("got error during exponential retry")
					s.logger.Error(uploadErr)
					err := f.Close()
					if err != nil {
						s.logger.Errorw("error closing file", "error", err)
					}
					return
				}
				if err := captureFile.Delete(); err != nil {
					s.logger.Error(err)
					err := f.Close()
					if err != nil {
						s.logger.Errorw("error closing file", "error", err)
					}
					return
				}
			}
		})
	}
}

//	s.backgroundWorkers.Add(1)
//	goutils.PanicCapturingGo(func() {
//		defer s.backgroundWorkers.Done()
//		for _, p := range paths {
//			select {
//			case <-s.cancelCtx.Done():
//				return
//			default:
//				//nolint:gosec
//				f, err := os.Open(p)
//				if err != nil {
//					s.logger.Errorw("error opening file", "error", err)
//					return
//				}
//
//				captureFile, err := datacapture.ReadFile(f)
//				if err != nil {
//					s.logger.Error(err)
//					err := f.Close()
//					if err != nil {
//						s.logger.Errorw("error closing file", "error", err)
//					}
//					return
//				}
//				uploadErr := exponentialRetry(
//					s.cancelCtx,
//					func(ctx context.Context) error {
//						return s.syncFile(ctx, s.client, captureFile, s.partID)
//					},
//					s.logger,
//				)
//				fmt.Println("exponential retry returned")
//				if uploadErr != nil {
//					fmt.Println("got error during exponential retry")
//					s.logger.Error(uploadErr)
//					err := f.Close()
//					if err != nil {
//						s.logger.Errorw("error closing file", "error", err)
//					}
//					return
//				}
//				if err := captureFile.Delete(); err != nil {
//					s.logger.Error(err)
//					err := f.Close()
//					if err != nil {
//						s.logger.Errorw("error closing file", "error", err)
//					}
//					return
//				}
//			}
//		}
//	})
//}

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

// TODO: add arbitrary files back.
func (s *syncer) syncFile(ctx context.Context, client v1.DataSyncServiceClient, f *datacapture.File, partID string) error {
	return uploadDataCaptureFile(ctx, client, f, partID)
}
