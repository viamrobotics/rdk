package grpc

import (
	"context"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

// AppConn maintains an underlying client connection meant to be used globally to connect to App. The AppConn constructor repeatedly
// attempts to dial App until a connection is successfully established.
type AppConn struct {
	ReconfigurableClientConn
}

// NewAppConn creates an AppConn instance with a gRPC client connection to App. An initial dial attempt blocks. If it errors, the error is
// returned. If it times out, an AppConn object will return with a nil underlying client connection. Serialized attempts at establishing a
// connection to App will continue to occur, however, in a background Goroutine. These attempts will continue until a connection is made or
// an error that is not a context.DeadlineExceeded occurs.
func NewAppConn(ctx context.Context, cloud *config.Cloud, logger logging.Logger) (*AppConn, error) {
	grpcURL, err := url.Parse(cloud.AppAddress)
	if err != nil {
		return nil, err
	}

	dialOpts := dialOpts(cloud)

	if grpcURL.Scheme == "http" {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}

	appConn := &AppConn{}

	ctxWithTimeout, ctxWithTimeoutCancel := config.GetTimeoutCtx(ctx, true, cloud.ID)
	defer ctxWithTimeoutCancel()

	// a lock is not necessary here because this call is blocking
	appConn.conn, err = rpc.DialDirectGRPC(ctxWithTimeout, grpcURL.Host, logger, dialOpts...)
	if err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		go func() {
			for {
				if ctx.Err() != nil {
					break
				}

				appConn.connMu.Lock()

				ctxWithTimeOut, ctxWithTimeOutCancel := context.WithTimeout(ctx, 5*time.Second)
				appConn.conn, err = rpc.DialDirectGRPC(ctxWithTimeOut, grpcURL.Host, logger, dialOpts...)
				ctxWithTimeOutCancel()
				if err != nil {
					logger.Debug("error while dialing App. Could not establish global, unified connection", err)
					appConn.connMu.Unlock()

					continue
				}

				break
			}

			appConn.connMu.Unlock()
		}()
	}

	return appConn, nil
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
