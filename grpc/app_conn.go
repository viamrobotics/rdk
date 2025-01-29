package grpc

import (
	"context"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

// AppConn maintains an underlying client connection meant to be used globally to connect to App. The `AppConn` constructor repeatedly
// attempts to dial App until a connection is successfully established.
type AppConn struct {
	ReconfigurableClientConn

	dialer *utils.StoppableWorkers
}

// NewAppConn creates an `AppConn` instance with a gRPC client connection to App. An initial dial attempt blocks. If it errors, the error
// is returned. If it times out, an `AppConn` object with a nil underlying client connection will return. Serialized attempts at establishing
// a connection to App will continue to occur, however, in a background Goroutine. These attempts will continue until a connection is made.
// If `cloud` is nil, an `AppConn` with a nil underlying connection will return, and the background dialer will not start.
func NewAppConn(ctx context.Context, cloud *config.Cloud, logger logging.Logger) (*AppConn, error) {
	appConn := &AppConn{}

	if cloud == nil {
		return appConn, nil
	}

	grpcURL, err := url.Parse(cloud.AppAddress)
	if err != nil {
		return nil, err
	}

	dialOpts := dialOpts(cloud)

	if grpcURL.Scheme == "http" {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}

	ctxWithTimeout, ctxWithTimeoutCancel := config.GetTimeoutCtx(ctx, true, cloud.ID)
	defer ctxWithTimeoutCancel()

	// lock not necessary here because call is blocking
	appConn.conn, err = rpc.DialDirectGRPC(ctxWithTimeout, grpcURL.Host, logger, dialOpts...)
	if err == nil {
		return appConn, nil
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}

	appConn.dialer = utils.NewStoppableWorkers(ctx)

	appConn.dialer.Add(func(ctx context.Context) {
		for {
			if ctx.Err() != nil {
				return
			}

			ctxWithTimeout, ctxWithTimeoutCancel := context.WithTimeout(ctx, 5*time.Second)
			appConn.connMu.Lock()
			appConn.conn, err = rpc.DialDirectGRPC(ctxWithTimeout, grpcURL.Host, logger, dialOpts...)
			appConn.connMu.Unlock()
			ctxWithTimeoutCancel()
			if err != nil {
				logger.Debug("error while dialing App. Could not establish global, unified connection", err)

				continue
			}

			return
		}
	})

	// if initial dial attempt fails due to time out, return nil error
	return appConn, nil
}

// Close attempts to close the underlying connection and stops background dialing attempts.
func (ac *AppConn) Close() error {
	if ac.dialer != nil {
		ac.dialer.Stop()
	}

	return ac.ReconfigurableClientConn.Close()
}

func dialOpts(cloud *config.Cloud) []rpc.DialOption {
	dialOpts := make([]rpc.DialOption, 0, 2)
	// Only add credentials when secret is set.
	if cloud.Secret != "" {
		dialOpts = append(dialOpts, rpc.WithEntityCredentials(cloud.ID,
			rpc.Credentials{
				Type:    "robot-secret",
				Payload: cloud.Secret,
			},
		))
	}
	return dialOpts
}
